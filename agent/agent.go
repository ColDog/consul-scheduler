package agent

import (
	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/executors"
	"github.com/shirou/gopsutil/mem"
	"github.com/syndtr/goleveldb/leveldb/errors"

	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

/**
 * An agent is the process that handles keeping a given host in sync with the desired configuration.
 * There are
 */

type AgentConfig struct {
	Runners      int           `json:"runners"`
	SyncInterval time.Duration `json:"sync_interval"`
}

const RUNNERS = 3

var (
	PassOnStartReqErr = errors.New("start request intervals too short")
	NoExecutorErr     = errors.New("start task failed")
)

type action struct {
	start bool
	stop  bool
	task  Task
}

func (a action) name() string {
	if a.start {
		return "start"
	} else {
		return "stop"
	}
}

func NewAgent(a *SchedulerApi) *Agent {
	return &Agent{
		api:       a,
		queue:     make(chan action, 100),
		stopCh:    make(chan struct{}, 1),
		StartedAt: make(map[string]time.Time),
		syncCh:    make(chan struct{}, 100),
		listenCh:  make(chan struct{}, 100),
		lock:      &sync.RWMutex{},
	}
}

type Agent struct {
	api       *SchedulerApi
	Host      string
	LastSync  time.Time
	LastState Host
	StartedAt map[string]time.Time

	lock     *sync.RWMutex
	queue    chan action
	stopCh   chan struct{}
	syncCh   chan struct{}
	listenCh chan struct{}
}

// starts a task and registers it into consul if the exec command returns a non-zero exit code
func (agent *Agent) start(t Task) error {

	agent.lock.RLock()
	if startTime, ok := agent.StartedAt[t.Id()]; ok {
		// if we add 3 minutes to the started at time, and it's after the current
		// time we skip this loop
		if startTime.Add(1 * time.Minute).After(time.Now()) {
			return PassOnStartReqErr
		}
	}
	agent.lock.RUnlock()

	agent.lock.Lock()
	agent.StartedAt[t.Id()] = time.Now()
	agent.lock.Unlock()

	for _, cont := range t.TaskDef.Containers {
		executor := executors.GetExecutor(cont)

		if executor != nil {
			err := executor.StartTask(t)
			if err != nil {
				agent.stop(t)
				return err
			}
		} else {
			agent.stop(t)
			return NoExecutorErr
		}
	}

	agent.api.Register(t)
	return nil
}

// deregisters a task from consul and attemps to stop the task from runnning
func (agent *Agent) stop(t Task) error {
	agent.api.DeRegister(t.Id())

	for _, cont := range t.TaskDef.Containers {
		executor := executors.GetExecutor(cont)

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
	agent.LastState = h

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

	agent.LastSync = time.Now()
}

func (agent *Agent) Stop() {
	close(agent.stopCh)
}

// the agent will sync every 30 seconds or when a value is passed over the syncCh. There are watcher processes which
// keep an eye one changes in consul and tell the syncer when to run the sync function.
func (agent *Agent) Run() {
	log.Info("[agent] starting")

	// the server provides a basic health checking port to allow for the agent to provide consul with updates
	go agent.Server()
	defer agent.api.DelHost(agent.Host)
	defer agent.api.DeRegisterAgent(agent.Host)

	for {
		name, err := agent.api.Host()
		if err == nil {
			agent.Host = name
			break
		}

		log.WithField("error", err).Error("[agent] could not get host")
		time.Sleep(5 * time.Second)
	}

	for {
		select {
		case <-agent.stopCh:
			log.Warn("[agent] exiting")
			return
		default:
		}

		err := agent.api.RegisterAgent(agent.Host)
		if err == nil {
			log.Info("[agent] registered agent")
			break
		}

		log.WithField("error", err).Error("[agent] could not register")
		time.Sleep(5 * time.Second)
	}

	log.Info("[agent] publishing initial state")

	for i := 0; i < RUNNERS; i++ {
		go agent.runner(i)
	}

	go agent.stateWatcher()

	log.Debug("[agent] waiting")
	for {
		select {
		case <-agent.syncCh:
			log.Debug("[agent] sync triggered")
			agent.sync()
			agent.PublishState()

		case <-time.After(30 * time.Second):
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
func (agent *Agent) Server() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := json.Marshal(agent)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// todo: make this useful!
		w.Write([]byte("OK\n"))
	})

	log.Fatal(http.ListenAndServe(":8231", nil))
}

// the runner handles starting processes. It listens to a queue of processes to start and starts them as needed.
// it will only start a task if it hasn't already attemted to start a task in the last minute.
func (agent *Agent) runner(id int) {
	for {
		select {
		case act := <-agent.queue:
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

// the watcher handles watching the state changes, it will emit messages to the syncCh telling the syncer to resync if
// there are any changes in the configuration.
func (agent *Agent) stateWatcher() {
	for {
		// exit if stopped
		select {
		case <-agent.stopCh:
			return
		default:
		}

		// watch the state plus this hosts host, meaning if the state for this host changes, the query
		// will return.
		err := agent.api.WaitOnKey(StatePrefix + agent.Host)
		if err != nil {
			time.Sleep(15 * time.Second)
			continue
		}

		// trigger a sync if the watcher is triggered.
		agent.syncCh <- struct{}{}
	}
}
