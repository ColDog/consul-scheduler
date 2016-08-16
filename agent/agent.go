package agent

import (
	"os/exec"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"
	"github.com/shirou/gopsutil/mem"
	"os"
	"github.com/coldog/scheduler/tools"
)

type action struct {
	start  bool
	stop   bool
	task   Task
	taskId string
}

func NewAgent(a *SchedulerApi) *Agent {
	return &Agent{
		api:       a,
		run:       make(chan struct{}, 1),
		queue:     make(chan action, 100),
		stopCh:    make(chan struct{}, 1),
		startedAt: make(map[string]time.Time),
	}
}

type Agent struct {
	api       *SchedulerApi
	Host      string
	startedAt map[string]time.Time
	events    *tools.Events
	queue     chan action
	stopCh    chan struct{}
	run       chan struct{}
}

func (agent *Agent) runner(id int) {
	for {
		select {
		case act := <-agent.queue:
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

		case <-agent.quit:
			return
		}
	}
}

// starts a task and registers it into consul if the exec command returns a non-zero exit code
func (agent *Agent) start(t Task) {
	for _, cont := range t.TaskDef.Containers {
		executor := cont.GetExecutor()

		if executor != nil {
			err := executor.Start(t)

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

// deregisters a task from consul and attemps to stop the task from runnning
func (agent *Agent) stop(t Task) {
	agent.api.DeRegister(t.Id())

	for _, cont := range t.TaskDef.Containers {
		executor := cont.GetExecutor()

		if executor != nil {
			env := executor.GetEnv(t)
			for _, cmd := range executor.StopCmds(t) {
				Exec(env, cmd[0], cmd[1:]...)
			}
		}
	}
}

func (agent *Agent) watcher() {
	for {
		err := agent.api.WaitOnKey("state/" + agent.Host)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		agent.run <- struct{}{}
	}
}

func (agent *Agent) Stop() {
	agent.quit <- struct{}{}
}

func (agent *Agent) Run() {
	log.Info("[agent] starting")

	go Serve()
	defer agent.api.DelHost(agent.Host)

	for {
		host, err := agent.api.Host()
		if err == nil {
			agent.Host = host
			break
		}

		log.WithField("error", err).Error("[agent] could not find host name")
		time.Sleep(5 * time.Second)
	}

	for {
		err := agent.api.RegisterAgent(agent.Host)
		if err == nil {
			break
		}

		log.WithField("error", err).Error("[agent] could not register")
		time.Sleep(5 * time.Second)
	}

	log.Info("[agent] publishing initial state")
	agent.publishState()

	for i := 0; i < 1; i++ {
		go agent.runner(i)
	}

	go agent.watcher()

	time.Sleep(5 * time.Second)
	agent.sync()

	for {
		select {

		case <-agent.run:
			log.Info("[agent] sync triggered by watcher")
			agent.sync()

		case <-time.After(45 * time.Second):
			agent.sync()

		case <-agent.quit:
			log.Info("[agent] exiting")
			return
		}
	}
}

// The agent periodically publishes it's state to consul so that the scheduler can use the latest information to
// make scheduling decisions.
func (agent *Agent) publishState() {
	m, _ := mem.VirtualMemory()

	desired, _ := agent.api.DesiredTasksByHost(agent.Host)
	ports := make([]uint, 0)

	for _, task := range desired {
		ports = append(ports, task.Port)
	}

	h := Host{
		Name:          agent.Host,
		Memory:        m.Available,
		CpuUnits:      uint64(runtime.NumCPU()),
		ReservedPorts: ports,
		PortSelection: AvailablePortList(20),
	}

	agent.api.PutHost(h)
}

// This function syncs the agent with consul and the provided state from the scheduler it compares the desired tasks for
// this host and the actual state from consul and adds to the queue new tasks to be started.
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

	// go through all of the desired tasks and check if they both and exist and are passing their health checks in
	// consul. If they are not, we push the task onto the queue.
	for _, task := range desired {
		runTask, ok := running[task.Id()]

		if ok && runTask.Exists && runTask.Passing {
			continue // continue if task is ok
		}

		log.WithField("task", task.Id()).WithField("passing", runTask.Passing).WithField("exists", runTask.Exists).WithField("ok", ok).Debug("[agent] enqueue")
		agent.queue <- action{
			start: true,
			task:  task,
		}
	}

	// go through all of the running tasks and check if they exist in the configuration. The agent is allowed to
	// stop a task when the running count for the tasks service is greater than the minimum.
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
	log.WithField("time", t2-t1).Info("[agent] finished sync")
	agent.publishState()
}
