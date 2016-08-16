package agent

import (
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"
	"github.com/shirou/gopsutil/mem"
	"github.com/coldog/scheduler/tools"
	"github.com/coldog/scheduler/executors"
	"sync"
)

const RUNNERS = 3

type action struct {
	start  bool
	stop   bool
	task   Task
	taskId string
}

func NewAgent(a *SchedulerApi) *Agent {
	return &Agent{
		api:       a,
		queue:     make(chan action, 100),
		stopCh:    make(chan struct{}, 1),
		startedAt: make(map[string]time.Time),
		events:    tools.NewEvents(),
		run:       make(chan struct{}, 100),
		lock:      &sync.RWMutex{},
	}
}

type Agent struct {
	api       *SchedulerApi
	Host      string
	startedAt map[string]time.Time
	events    *tools.Events
	lock      *sync.RWMutex
	queue     chan action
	stopCh    chan struct{}
	run       chan struct{}
}

func (agent *Agent) runner(id int) {
	for {
		select {
		case act := <-agent.queue:
			if act.start {
				agent.lock.RLock()

				if t, ok := agent.startedAt[act.task.Id()]; ok {
					// if we add 3 minutes to the started at time, and it's after the current
					// time we skip this loop
					if t.Add(30 * time.Second).After(time.Now()) {
						log.Infof("[agent-r%d] passing on start request", id)
						continue
					}
				}
				agent.lock.RUnlock()

				log.WithField("task", act.task.Id()).Infof("[agent-r%d] starting task", id)

				agent.lock.Lock()
				agent.startedAt[act.task.Id()] = time.Now()
				agent.lock.Unlock()

				agent.start(act.task)
			} else if act.stop {
				log.WithField("task", act.task.Id()).Infof("[agent-r%d] stopping task", id)
				agent.stop(act.task)
			}

		case <-agent.stopCh:
			return
		}
	}
}

// the syncer responds to requests to sync a specific task
func (agent *Agent) syncer() {
	for {
		select {
		case <-agent.run:
			agent.sync()

		case <-agent.stopCh:
			return

		}
	}
}

// starts a task and registers it into consul if the exec command returns a non-zero exit code
func (agent *Agent) start(t Task) {
	for _, cont := range t.TaskDef.Containers {
		executor := executors.GetExecutor(cont)

		if executor != nil {
			err := executor.StartTask(t)

			if err != nil {
				agent.stop(t)
				log.WithField("error", err).Error("[agent] failed to start task")
				return
			}
		} else {
			log.WithField("error", "no executor").Error("[agent] no executor found")
		}
	}

	log.WithField("task", t.Id()).Info("[agent] started task")
	agent.api.Register(t)
}

// deregisters a task from consul and attemps to stop the task from runnning
func (agent *Agent) stop(t Task) {
	agent.api.DeRegister(t.Id())

	for _, cont := range t.TaskDef.Containers {
		executor := executors.GetExecutor(cont)

		if executor != nil {
			err := executor.StopTask(t)
			if err != nil {
				log.WithField("error", err).Error("[agent] failed to stop task")
			}
		}
	}
}

func (agent *Agent) watcher(prefix string) {
	for {
		err := agent.api.WaitOnKey(prefix)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		agent.events.Publish()
	}
}

func (agent *Agent) Stop() {
	agent.stopCh <- struct{}{}
}

// The main loop that the agent sits in
func (agent *Agent) Run() {
	log.Info("[agent] starting")

	listener := make(chan struct{})
	agent.events.Subscribe(agent.Host, listener)
	defer agent.events.UnSubscribe(agent.Host)
	defer close(listener)

	// the server provides a basic health checking port to allow for the agent to provide consul with updates
	go Serve()
	defer agent.api.DelHost(agent.Host)

	for {
		select {
		case <-agent.stopCh:
			log.Warn("[agent] exiting")
			return
		default:
		}

		err := agent.api.RegisterAgent(agent.Host)
		if err == nil {
			break
		}

		log.WithField("error", err).Error("[agent] could not register")
		time.Sleep(5 * time.Second)
	}

	log.Info("[agent] publishing initial state")
	agent.publishState()

	for i := 0; i < RUNNERS; i++ {
		go agent.runner(i)
	}

	go agent.watcher(StatePrefix+agent.Host)
	go agent.syncer()

	for {
		log.Debug("[agent] waiting")

		select {

		case <-listener:
			log.Debug("[agent] sync triggered by watcher")
			agent.run <- struct {}{}

		case <-time.After(30 * time.Second):
			agent.run <- struct {}{}

		case <-agent.stopCh:
			log.Warn("[agent] exiting")
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
