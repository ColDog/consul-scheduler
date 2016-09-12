package agent

import (
	log "github.com/Sirupsen/logrus"
	"github.com/shirou/gopsutil/mem"

	"github.com/coldog/sked/api"
	"github.com/coldog/sked/config"

	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
	"github.com/coldog/sked/tools"
)

type AgentConfig struct {
	Runners      int            `json:"runners"`
	SyncInterval tools.Duration `json:"sync_interval"`
	AppConfig    *config.Config `json:"app_config"`
	Resources    *api.Resources `json:"resources"`
	Cluster      string         `json:"cluster"`
}

var (
	NoExecutorErr = errors.New("start task failed")
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

	if conf.SyncInterval.IsNone() {
		conf.SyncInterval.Duration = 30 * time.Second
	}

	m, _ := mem.VirtualMemory()
	d, _ := AvailableDiskSpace()

	if conf.Resources == nil {
		conf.Resources = &api.Resources{}
	}

	if conf.Resources.Memory == 0 {
		conf.Resources.Memory = ToMb(int64(m.Available))
	}

	if conf.Resources.DiskSpace == 0 {
		conf.Resources.DiskSpace = ToMb(d)
	}

	if conf.Resources.CpuUnits == 0 {
		conf.Resources.CpuUnits = int64(runtime.NumCPU() * 1024)
	}

	return &Agent{
		api:     a,
		queue:   make(chan action, 500),
		stopCh:  make(chan struct{}, 1),
		readyCh: make(chan struct{}),
		Config:  conf,
		TaskState: &AgentState{
			State: make(map[string]*TaskState),
			l:     &sync.RWMutex{},
		},
	}
}

type Agent struct {
	api       api.SchedulerApi
	Host      string              `json:"host"`
	LastSync  time.Time           `json:"last_sync"`
	LastState *api.Host           `json:"last_state"`
	TaskState *AgentState         `json:"task_state"`
	Config    *AgentConfig        `json:"config"`
	queue     chan action
	stopCh    chan struct{}
	readyCh   chan struct{}
}

// starts a task and registers it into consul if the exec command returns a non-zero exit code
func (agent *Agent) start(t *api.Task) error {
	err := agent.api.PutTaskState(t.ID(), api.STARTING)
	if err != nil {
		return err
	}

	state := agent.TaskState.get(t.ID(), t)
	state.Starting = true
	state.StartedAt = time.Now()
	state.Attempts += 1

	for _, cont := range t.TaskDefinition.Containers {
		executor := cont.GetExecutor()

		if executor != nil {
			cont.RunSetup()
			err := executor.StartTask(t)
			if err != nil {
				state.Failure = err
				return err
			}
		} else {
			state.Failure = NoExecutorErr
			return NoExecutorErr
		}
	}

	state.Starting = false
	return agent.api.PutTaskState(t.ID(), api.STARTING)
}

// deregisters a task from consul and attemps to stop the task from runnning
func (agent *Agent) stop(t *api.Task) error {
	err := agent.api.PutTaskState(t.ID(), api.STOPPING)
	if err != nil {
		return err
	}

	for _, cont := range t.TaskDefinition.Containers {
		executor := cont.GetExecutor()

		if executor != nil {
			err := executor.StopTask(t)
			if err != nil {
				return err
			}

			cont.RunTeardown()
		}
	}

	// delete the task from the state.
	agent.api.DelTask(t)
	agent.TaskState.del(t.Id())

	return agent.api.PutTaskState(t.ID(), api.STOPPED)
}

// The agent periodically publishes it's state to consul so that the scheduler can use the latest information to
// make scheduling decisions.
func (agent *Agent) PublishState() {

	var max uint
	reserved := make([]uint, 0)

	usedMem := int64(0)
	usedDisk := int64(0)
	usedCpu := int64(0)

	agent.TaskState.each(func(t *TaskState) error {
		for _, p := range t.Task.TaskDefinition.AllPorts() {
			if p == 0 {
				continue
			}

			reserved = append(reserved, p)
			if p > max {
				max = p + 1
			}
		}

		counts := t.Task.TaskDefinition.Counts()

		usedMem += counts.Memory
		usedDisk += counts.DiskUse
		usedCpu += counts.CpuUnits

		return nil
	})

	if agent.LastState == nil {
		agent.LastState = &api.Host{
			Name:                agent.Host,
			HealthCheck:         "http://" + agent.Config.AppConfig.Advertise + "/agent/health",
			CalculatedResources: &api.Resources{},
			BaseResources:       agent.Config.Resources,
			ObservedResources:   &api.Resources{},
		}
	}

	agent.LastState.CalculatedResources.Memory = agent.Config.Resources.Memory - usedMem
	agent.LastState.CalculatedResources.DiskSpace = agent.Config.Resources.DiskSpace - usedDisk
	agent.LastState.CalculatedResources.CpuUnits = agent.Config.Resources.CpuUnits - usedCpu
	agent.LastState.ReservedPorts = reserved

	m, _ := mem.VirtualMemory()
	d, _ := AvailableDiskSpace()

	agent.LastState.ObservedResources.Memory = ToMb(int64(m.Available))
	agent.LastState.ObservedResources.DiskSpace = ToMb(d)
	agent.LastState.ObservedResources.CpuUnits = int64(runtime.NumCPU() * 1024)

	agent.api.PutHost(agent.LastState)
}

func (agent *Agent) syncTask(task *api.Task) *action {
	taskState, err := agent.api.GetTaskState(task)
	if err != nil {
		log.WithField("error", err).Error("failed to sync")
		return nil
	}

	state := agent.TaskState.get(task.Id(), task)
	state.Healthy = taskState.Healthy()
	if state.Healthy {
		state.Failure = nil
	}

	deployment, err := agent.api.GetDeployment(task.Deployment)
	if err == api.ErrNotFound {
		task.RejectReason = "no deployment found"
		task.Rejected = true
		state.Failure = errors.New(task.RejectReason)
		agent.api.PutTask(task)
		log.WithField("task", task.ID()).Warn("[agent] no deployment found")
		return nil
	}

	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"task":         task.ID(),
		"attempts":     state.Attempts,
		"failure":      state.Failure,
		"health":       state.Healthy,
		"scheduled":    task.Scheduled,
		"last_started": state.StartedAt,
	}).Info("[agent] task state")

	// we start a task if the following conditions hold:
	// - the task is scheduled
	// - the task is not rejected
	// - the task is not healthy and has health checks or the task has not been started and does not have any checks
	if task.Scheduled && !task.Rejected && !state.Starting && ((!taskState.Healthy() && task.HasChecks()) || (!task.HasChecks() && state.Attempts == 0)) {
		// leave some time in between restarting tasks, ie they may take a while before the health checks
		// begin passing.
		if state.StartedAt.Add(task.TaskDefinition.GracePeriod.Duration).After(time.Now()) {
			log.Debug("[agent] skipping start, too short an interval")
			return nil
		}

		if state.Attempts > deployment.MaxAttempts {
			task.RejectReason = "too many attempts"
			task.Rejected = true
			state.Failure = errors.New(task.RejectReason)
			agent.api.PutTask(task)
			log.WithField("task", task.Id()).Warn("[agent] too many attempts")
			return nil
		}

		// Check if there is a port conflict. This actually attempts to bind to the port which gives us a better
		// picture overall. This can be problematic if the task is not shutdown correctly
		for _, p := range task.TaskDefinition.AllPorts() {
			if !IsTCPPortAvailable(p) {
				task.RejectReason = fmt.Sprintf("port not available: %d", p)
				task.Rejected = true
				state.Failure = errors.New(task.RejectReason)
				agent.api.PutTask(task)
				log.WithField("task", task.Id()).WithField("port", p).Warn("[agent] port not available")
				return nil
			}
		}

		log.WithField("last_started", state.StartedAt).WithField("task", task.Id()).Info("[agent] starting")
		state.StartedAt = time.Now() // put the started at here since time into the queue could be longer.

		// restart the task since it is failing
		return &action{start: true, task: task}
	}

	if !task.Scheduled {
		// stop the task if it's not scheduled
		log.Debug("[agent] triggering stop")
		return &action{stop: true, task: task}
	}

	// do nothing
	return nil
}

// This function syncs the agent with consul and the provided state from the scheduler it compares the desired tasks for
// this host and the actual state from consul and adds to the queue new tasks to be started.
func (agent *Agent) sync() {
	t1 := time.Now().UnixNano()

	tasks, err := agent.api.ListTasks(&api.TaskQueryOpts{
		ByHost: agent.Host,
		ByCluster: agent.Config.Cluster,
	})
	if err != nil {
		log.WithField("error", err).Error("failed to sync")
		return
	}

	for _, task := range tasks {
		act := agent.syncTask(task)

		if act != nil {
			select {
			case agent.queue <- *act:
			case <-time.After(15 * time.Second):
				log.Error("[agent] queue is full")
			}
		}
	}

	t2 := time.Now().UnixNano()
	log.WithField("time", t2-t1).WithField("secs", float64(t2-t1)/1000000000.0).Info("[agent] finished sync")

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

	agent.TaskState.load()
	defer agent.TaskState.save()

	// the server provides a basic health checking port to allow for the agent to provide consul with updates
	defer agent.api.DelHost(agent.Host)

	agent.GetHostName()
	agent.PublishState()

	for i := 0; i < agent.Config.Runners; i++ {
		go agent.runner(i)
	}

	listenState := make(chan string)
	agent.api.Subscribe("agent-state", "state::state/hosts/"+agent.Host, listenState)
	defer agent.api.UnSubscribe("agent-state")
	defer close(listenState)

	listenHealth := make(chan string)
	agent.api.Subscribe("agent-health", "health::task:"+agent.Host+":failing:*", listenHealth)
	defer agent.api.UnSubscribe("agent-health")
	defer close(listenHealth)

	close(agent.readyCh)
	agent.sync()

	log.Debug("[agent] waiting")
	for {
		select {
		case x := <-listenState:
			log.WithField("key", x).Info("[agent] sync triggered")
			agent.sync()
			agent.PublishState()

		case x := <-listenHealth:
			log.WithField("key", x).Info("[agent] sync triggered")
			agent.sync()
			agent.PublishState()

		case <-time.After(agent.Config.SyncInterval.Duration):
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
	log.Infof("[agent-runner-%d] starting", id)

	for {
		select {
		case act := <-agent.queue:
			log.WithField("task", act.task.Id()).Debugf("[agent-runner-%d] begin %s", id, act.name())

			var err error
			if act.start {
				err = agent.start(act.task)
			} else if act.stop {
				err = agent.stop(act.task)
			}

			if err != nil {
				log.WithField("err", err).Warnf("[agent-runner-%d] failure to %s task", id, act.name())
			}

		case <-agent.stopCh:
			log.Warnf("[agent-runner-%d] exiting", id)
			return
		}
	}
}
