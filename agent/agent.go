package agent

import (
	"time"
	"os/exec"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/shirou/gopsutil/mem"
	. "github.com/coldog/scheduler/api"
	"os"
)

type action struct {
	start		bool
	stop 		bool
	task 		Task
	taskId		string
}

func NewAgent(a *SchedulerApi) *Agent {
	log.Info("[agent] starting agent")
	return &Agent{
		api: a,
		Host: a.Host(),
		run: make(chan struct{}, 1),
		queue: make(chan action, 100),
		quit: make(chan struct{}, 1),
		startedAt: make(map[string] time.Time),
	}
}

type Agent struct {
	api 		*SchedulerApi
	Host 		string
	startedAt 	map[string] time.Time
	queue 		chan action
	quit 		chan struct{}
	run 		chan struct{}
}

func (agent *Agent) availablePortList() []uint {
	ports := make([]uint, 0, 60)
	for i := 0; i < 20; i++ {
		ports = append(ports, uint(RandomTCPPort()))
	}
	return ports
}

func (agent *Agent) runner(id int) {
	for {
		select {
		case act := <- agent.queue:
			if act.start {

				if t, ok := agent.startedAt[act.task.Id()]; ok {
					// if we add 3 minutes to the started at time, and it's after the current
					// time we skip this loop
					if t.Add(30 * time.Second).After(time.Now()) {
						log.Infof("[agent-r%d] passing on start request", id)
						continue
					}
				}

				log.WithField("task", act.task.Id()).Infof("[agent-r%d] starting task", id)
				agent.startedAt[act.task.Id()] = time.Now()
				agent.start(act.task)
			} else if act.stop {
				log.WithField("task", act.task.Id()).Infof("[agent-r%d] stopping task", id)
				agent.stop(act.task)
			}

		case <- agent.quit:
			return
		}
	}
}

func (agent *Agent) exec(env []string, main string, cmds ...string) error {
	log.WithField("cmd", main).WithField("args", cmds).WithField("env", env).Info("[agent] executing")

	done := make(chan struct{}, 1)
	cmd := exec.Command(main, cmds...)
	cmd.Env = env

	go func() {

		select {
		case <- done:
			return
		case <- time.After(30 * time.Second):
			cmd.Process.Kill()
			return
		}

	}()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		done <- struct{}{}
		return err
	}


	err = cmd.Wait()
	done <- struct{}{}
	return err
}

func (agent *Agent) start(t Task) {
	for _, cont := range t.TaskDef.Containers {
		var err error
		executor := cont.GetExecutor()

		if executor != nil {
			env := executor.GetEnv(t)
			cmds := executor.StartCmds(t)
			for _, cmd := range cmds {
				err = agent.exec(env, cmd[0], cmd[1:]...)
			}

			if err != nil {
				// task has failed to start if the final command returns and error, we will exit
				// the loop and not register the service
				log.WithField("error", err).Error("[agent] failed to start task")
				return
			}
		}
	}

	log.WithField("task", t.Id()).Info("[agent] started task")
	agent.api.Register(t)
}

func (agent *Agent) stop(t Task) {
	agent.api.DeRegister(t.Id())

	for _, cont := range t.TaskDef.Containers {
		executor := cont.GetExecutor()

		if executor != nil {
			env := executor.GetEnv(t)
			for _, cmd := range executor.StopCmds(t) {
				agent.exec(env, cmd[0], cmd[1:]...)
			}
		}
	}
}

func (agent *Agent) watcher() {
	for {
		agent.api.WaitOnKey("state/" + agent.Host)
		agent.run <- struct {}{}
	}
}

func (agent *Agent) Run() {
	log.Info("[agent] publishing initial state")
	agent.api.RegisterAgent(agent.Host)
	defer agent.api.DelHost(agent.Host)

	go Serve()

	agent.publishState()

	for i := 0; i < 1; i++ {
		go agent.runner(i)
	}

	go agent.watcher()

	time.Sleep(5 * time.Second)
	agent.sync()

	for {
		select {

		case <- agent.run:
			log.Info("[agent] sync triggered by watcher")
			agent.sync()

		case <- time.After(45 * time.Second):
			agent.sync()

		case <- agent.quit:
			return
		}
	}
}

func (agent *Agent) publishState() {
	m, _ := mem.VirtualMemory()

	desired, _ := agent.api.DesiredTasksByHost(agent.Host)
	ports := make([]uint, 0)

	for _, task := range desired {
		ports = append(ports, task.Port)
	}

	h := Host{
		Name: agent.Host,
		Memory: m.Available,
		CpuUnits: uint64(runtime.NumCPU()),
		ReservedPorts: ports,
		PortSelection: agent.availablePortList(),
	}

	agent.api.PutHost(h)
}

func (agent *Agent) sync() {
	t1 := time.Now().UnixNano()

	desired, err := agent.api.DesiredTasksByHost(agent.Host)
	if err != nil {
		log.WithField("error", err).Error("failed to sync")
		return
	}

	running, err := agent.api.RunningTasksOnHost(agent.Host)
	if err != nil {
		log.WithField("error", err).Error("failed to sync")
		return
	}


	for _, task := range desired {
		runTask, ok := running[task.Id()]

		if ok && runTask.Exists && runTask.Passing {
			continue // continue if task is ok
		}

		log.WithField("task", task.Id()).WithField("passing", runTask.Passing).WithField("exists", runTask.Exists).WithField("ok", ok).Debug("[agent] enqueue")

		agent.queue <- action{
			start: true,
			task: task,
		}
	}

	for _, runTask := range running {

		if runTask.Task.Stopped {
			log.WithField("task", runTask.Task.Id()).Debug("[agent] task stopped")

			c, err := agent.api.HealthyTaskCount(runTask.Task.Name())
			if err != nil {
				log.WithField("error", err).Error("failed to sync")
				return
			}

			if c > runTask.Service.Min {
				agent.queue <- action{
					stop: true,
					task: runTask.Task,
				}
			}
		}

		// no way to stop the task since we cannot find it, instead we deregister in consul
		if !runTask.Exists {
			log.WithField("task", runTask.ServiceID).Debug("[agent] task doesn't exist")
			agent.api.DeRegister(runTask.ServiceID)
		}
	}

	t2 := time.Now().UnixNano()
	log.WithField("time", t2 - t1).Info("[agent] finished sync")
	agent.publishState()
}
