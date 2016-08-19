package agent

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/scheduler/api"
	"github.com/shirou/gopsutil/mem"

	"errors"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

/**
 * An agent is the process that handles keeping a given host in sync with the desired configuration.
 * An agent
 */

type AgentConfig struct {
	Runners      int           `json:"runners"`
	SyncInterval time.Duration `json:"sync_interval"`
	Port         int           `json:"port"`
	Addr         string        `json:"addr"`
}

var (
	PassOnStartReqErr = errors.New("start request intervals too short")
	NoExecutorErr     = errors.New("start task failed")
)

type action struct {
	start bool
	stop  bool
	task  *api.Task
}

func (a action) name() string {
	if a.start {
		return "start"
	} else {
		return "stop"
	}
}

func NewAgent(a api.SchedulerApi, conf *AgentConfig) *Agent {

	if conf.Runners == 0 {
		conf.Runners = 3
	}

	if conf.SyncInterval.Nanoseconds() == int64(0) {
		conf.SyncInterval = 30 * time.Second
	}

	return &Agent{
		api:       a,
		queue:     make(chan action, 100),
		stopCh:    make(chan struct{}, 1),
		StartedAt: make(map[string]time.Time),
		readyCh:   make(chan struct{}),
		lock:      &sync.RWMutex{},
		Config:    conf,
	}
}

type Agent struct {
	api       api.SchedulerApi
	Host      string
	LastSync  time.Time
	LastState *api.Host
	StartedAt map[string]time.Time
	Config    *AgentConfig
	lock      *sync.RWMutex
	queue     chan action
	stopCh    chan struct{}
	readyCh   chan struct{}
}

// starts a task and registers it into consul if the exec command returns a non-zero exit code
func (agent *Agent) start(t *api.Task) error {

	agent.lock.RLock()
	if startTime, ok := agent.StartedAt[t.Id()]; ok {
		// if we add 3 minutes to the started at time, and it's after the current
		// time we skip this loop
		if startTime.Add(15 * time.Second).After(time.Now()) {
			agent.lock.RUnlock()
			return PassOnStartReqErr
		}
	}
	agent.lock.RUnlock()

	agent.lock.Lock()
	agent.StartedAt[t.Id()] = time.Now()
	agent.lock.Unlock()

	for _, cont := range t.TaskDefinition.Containers {
		executor := api.GetExecutor(cont)

		if executor != nil {
			err := executor.StartTask(t)
			if err != nil {
				return err
			}
		} else {
			return NoExecutorErr
		}
	}

	agent.api.Register(t)
	return nil
}

// deregisters a task from consul and attemps to stop the task from runnning
func (agent *Agent) stop(t *api.Task) error {
	agent.api.DeRegister(t.Id())

	for _, cont := range t.TaskDefinition.Containers {
		executor := api.GetExecutor(cont)

		if executor != nil {
			err := executor.StopTask(t)
			if err != nil {
				return err
			}
		}
	}

	if _, ok := agent.StartedAt[t.Id()]; ok {
		agent.lock.Lock()
		delete(agent.StartedAt, t.Id())
		agent.lock.Unlock()
	}

	return nil
}

// The agent periodically publishes it's state to consul so that the scheduler can use the latest information to
// make scheduling decisions.
func (agent *Agent) PublishState() {
	m, _ := mem.VirtualMemory()

	desired, _ := agent.api.ListTasks(&api.TaskQueryOpts{
		ByHost: agent.Host,
		Scheduled: true,
	})
	ports := make([]uint, 0)

	for _, task := range desired {
		ports = append(ports, task.Port)
	}

	h := &api.Host{
		Name:          agent.Host,
		Memory:        m.Available,
		CpuUnits:      uint64(runtime.NumCPU()),
		ReservedPorts: ports,
		PortSelection: AvailablePortList(20),
	}

	agent.LastState = h
	agent.api.PutHost(h)
}

// This function syncs the agent with consul and the provided state from the scheduler it compares the desired tasks for
// this host and the actual state from consul and adds to the queue new tasks to be started.
func (agent *Agent) sync() {
	t1 := time.Now().UnixNano()

	tasks, err := agent.api.ListTasks(&api.TaskQueryOpts{
		ByHost: agent.Host,
	})
	if err != nil {
		log.WithField("error", err).Error("failed to sync")
		return
	}

	for _, task := range tasks {

		log.WithField("task", task.Id()).WithField("passing", task.Passing).WithField("scheduled", task.Scheduled).Debug("[agent] syncing task")

		if task.Scheduled && !task.Passing {
			log.Debug("[agent] triggering start!")
			// restart the task since it is failing
			select {
			case agent.queue <- action{start: true, task:  task}:
			default:
				log.Error("[agent] queue is full")
			}
		}

		if !task.Scheduled && task.Passing {
			log.Debug("[agent] triggering stop!")

			// stop the task since it shouldn't be running
			select {
			case agent.queue <- action{stop: true, task:  task}:
			default:
				log.Error("[agent] queue is full")
			}
		}

	}

	t2 := time.Now().UnixNano()
	log.WithField("time", t2-t1).Info("[agent] finished sync")

	agent.LastSync = time.Now()
}

// get our host name from consul, this will block.
func (agent *Agent) GetHostName() {
	for {
		select {
		case <-agent.stopCh:
			log.Warn("[agent] exiting")
			return
		default:
		}

		name, err := agent.api.HostName()
		if err == nil {
			agent.Host = name
			break
		}

		log.WithField("error", err).Error("[agent] could not get host")
		time.Sleep(5 * time.Second)
	}
}

// register the agent in consul, this will block.
func (agent *Agent) RegisterAgent() {
	for {
		select {
		case <-agent.stopCh:
			log.Warn("[agent] exiting")
			return
		default:
		}

		err := agent.api.RegisterAgent(agent.Host, agent.Config.Addr, agent.Config.Port)
		if err == nil {
			log.Info("[agent] registered agent")
			break
		}

		log.WithField("error", err).Error("[agent] could not register")
		time.Sleep(5 * time.Second)
	}
}

func (agent *Agent) Stop() {
	close(agent.stopCh)
}

func (agent *Agent) Wait() {
	<-agent.readyCh
}

// the agent will sync every 30 seconds or when a value is passed over the syncCh. There are watcher processes which
// keep an eye one changes in consul and tell the syncer when to run the sync function.
func (agent *Agent) Run() {
	log.Info("[agent] starting")

	// the server provides a basic health checking port to allow for the agent to provide consul with updates
	defer agent.api.DelHost(agent.Host)
	defer agent.api.DeRegister("consul-scheduler-"+agent.Host)

	agent.GetHostName()
	agent.RegisterAgent()

	for i := 0; i < agent.Config.Runners; i++ {
		go agent.runner(i)
	}

	listener := make(chan string)
	agent.api.Subscribe("agent", "*", listener)
	defer agent.api.UnSubscribe("agent")
	defer close(listener)

	select {
	case <-agent.stopCh:
		log.Warn("[agent] exiting")
		return
	default:
	}

	agent.PublishState()

	close(agent.readyCh)

	agent.sync()

	log.Debug("[agent] waiting")
	for {
		select {
		case <-listener:
			log.Debug("[agent] sync triggered")
			agent.sync()
			agent.PublishState()

		case <-time.After(agent.Config.SyncInterval):
			log.Debug("[agent] sync after timeout")
			agent.sync()
			agent.PublishState()

		case <-agent.stopCh:
			log.Warn("[agent] exiting")
			return

		}
	}
}

// the http server handles providing a health checking and informational http endpoint about the agent.
// the main health route is at /health and the root route provides basic statistics and information.
func (agent *Agent) RegisterRoutes() {
	http.HandleFunc("/agent/status", func(w http.ResponseWriter, r *http.Request) {
		res, err := json.Marshal(agent)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
	})

	http.HandleFunc("/agent/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK\n"))
	})
}

// the runner handles starting processes. It listens to a queue of processes to start and starts them as needed.
// it will only start a task if it hasn't already attemted to start a task in the last minute.
func (agent *Agent) runner(id int) {
	log.Debugf("[runner-%d] starting", id)

	for {
		select {
		case act := <-agent.queue:
			log.WithField("task", act.task.Id()).Debugf("[runner-%d] begin %s", id, act.name())

			var err error
			if act.start {
				err = agent.start(act.task)
			} else if act.stop {
				err = agent.stop(act.task)
			}

			if err != nil {
				log.WithField("err", err).Warnf("[runner-%d] failure to %s task", id, act.name())
			}

		case <-agent.stopCh:
			return
		}
	}
}
