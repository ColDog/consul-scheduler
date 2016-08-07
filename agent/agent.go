package agent

import (
	"time"
	"os/exec"
	"fmt"
	"bufio"
	"strings"
	"runtime"
	log "github.com/Sirupsen/logrus"
	"github.com/shirou/gopsutil/mem"
	. "github.com/coldog/scheduler/api"
)

type action struct {
	start		bool
	stop 		bool
	task 		Task
}

func NewAgent(a *SchedulerApi) *Agent {
	log.Info("starting agent")
	return &Agent{
		api: a,
		Host: a.Host(),
		queue: make(chan action, 100),
		quit: make(chan struct{}, 1),
	}
}

type Agent struct {
	api 		*SchedulerApi
	Host 		string
	pids 		map[string] string
	queue 		chan action
	quit 		chan struct{}
	run 		chan struct{}
}

func (agent *Agent) runner() {
	for {
		select {

		case act := <- agent.queue:
			if act.start {
				log.WithField("task", act.task.Id()).Info("[agent]	starting task")
				agent.start(act.task)
			} else if act.stop {
				log.WithField("task", act.task.Id()).Info("[agent]	stopping task")
				agent.stop(act.task)
			}

		case <- agent.quit:
			return
		}
	}
}

func (agent *Agent) exec(env []string, main string, cmds ...string) error {
	log.WithField("cmd", cmds).Info("[agent]	executing")

	done := make(chan bool, 1)
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

	// capture the output and error pipes
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()
	err = cmd.Start()
	if err != nil {
		fmt.Printf("	> err: %v\n", err.Error())
		return err
	}

	go func() {
		buff := bufio.NewScanner(stderr)

		for buff.Scan() {
			fmt.Printf("	> %s\n", buff.Text())
		}
	}()

	go func() {
		buff := bufio.NewScanner(stdout)

		for buff.Scan() {
			fmt.Printf("	> %s\n", buff.Text())
		}
	}()


	cmd.Wait()
	done <- true
	return nil
}

func (agent *Agent) start(t Task) {
	agent.api.Register(t)

	for _, cont := range t.TaskDef.Containers {
		log.Println("[agent]	starting", cont.Executor)
		switch cont.Executor {
		case "docker":
			for _, cmd := range cont.Docker.Commands() {
				agent.exec(cont.Docker.Env, cmd[0], cmd[1:]...)
			}
		case "bash":
			for _, cmd := range cont.Bash.Commands() {
				agent.exec(cont.Bash.Env, cmd[0], cmd[1:]...)
			}
		}
	}
}

func (agent *Agent) stop(t Task) {
	agent.api.DeRegister(t)

	for _, cont := range t.TaskDef.Containers {
		switch cont.Executor {
		case "docker":
			agent.exec([]string{}, "docker", "stop", cont.Docker.Name)

		case "bash":
			agent.exec([]string{}, cont.Bash.Kill)

		}
	}
}

func (agent *Agent) watcher() {
	for {
		agent.api.WaitOnKey("state/" + agent.Host + "/")
		agent.run <- struct {}{}
	}
}

func (agent *Agent) Run() {
	log.Info("[agent]	publishing initial state")

	agent.publishState()
	go agent.runner()
	go agent.watcher()

	for {
		select {

		case <- agent.run:
			agent.sync()

		case <- time.After(10 * time.Second):
			agent.sync()

		case <- agent.quit:
			return
		}
	}
}

func (agent *Agent) publishState() {
	m, _ := mem.VirtualMemory()

	h := Host{
		Name: agent.Host,
		Memory: m.Available,
		CpuUnits: uint64(runtime.NumCPU()),
	}

	agent.api.PutHost(h)
}

func (agent *Agent) sync() {
	log.Println("[agent] 	executing sync")

	desired := agent.api.DesiredTasksByHost(agent.Host)
	running := agent.api.RunningTasksOnHost(agent.Host)

	for _, task := range desired {
		runTask, ok := running[task.Id()]

		if ok && runTask.Exists && runTask.Passing {
			continue // continue if task is ok
		}

		log.WithField("task", task.Id()).WithField("passing", runTask.Passing).WithField("exists", runTask.Exists).WithField("ok", ok).Debug("[agent]	enqueue")

		agent.queue <- action{
			start: true,
			task: task,
		}
	}

	for _, runTask := range running {

		if !runTask.Exists {

			log.WithField("task", runTask.Task.Id()).Debug("[agent] 	task doesn't exist")

			// parse the service id and try and keep everything above the minimum
			spl := strings.Split(runTask.ServiceID, "_")
			if len(spl) >= 2 {
				serv, ok := agent.api.GetService(spl[1])
				if ok {
					c := agent.api.HealthyTaskCount(spl[1])

					if c > serv.Min {
						agent.queue <- action{
							stop: true,
							task: runTask.Task,
						}
					}
				}
			}

		}
	}

	agent.publishState()
}
