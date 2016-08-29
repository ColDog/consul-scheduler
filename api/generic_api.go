package api

import (
	"encoding/json"
)

type TaskState int

const (
	STOPPED TaskState = iota
	RUNNING
	FAILING
)

// any lockable interface should implement the same functionality.
type Lockable interface {
	Lock() (<-chan struct{}, error)
	QuitChan() <-chan struct{}
	IsHeld() bool
	Unlock() error
}

type StorageConfig struct {
	ConfigPrefix          string
	ClustersPrefix        string
	ServicesPrefix        string
	HostsPrefix           string
	TaskDefinitionsPrefix string
	SchedulersPrefix      string
	StatePrefix           string
	TasksPrefix           string
	TasksByHostPrefix     string
}

func DefaultStorageConfig() *StorageConfig {
	confPrefix := "config/"
	statePrefix := "state/"

	return &StorageConfig{
		ConfigPrefix:          confPrefix,
		ClustersPrefix:        confPrefix + "clusters/",
		ServicesPrefix:        confPrefix + "services/",
		HostsPrefix:           confPrefix + "hosts/",
		TaskDefinitionsPrefix: confPrefix + "task_definitions/",
		SchedulersPrefix:      "schedulers/",
		StatePrefix:           statePrefix,
		TasksByHostPrefix:     statePrefix + "hosts/",
		TasksPrefix:           statePrefix + "tasks/",
	}
}

type TaskQueryOpts struct {
	ByCluster string
	ByService string
	ByHost    string
	Running   bool
	Scheduled bool
	Failing   bool
	Rejected  bool
}

type SchedulerApi interface {
	HostName() (string, error)

	// acquire a lock on a resource from consul, does not block when the lock cannot be held, rather
	// it should return immediately
	Lock(key string, block bool) (Lockable, error)

	Wait() error
	Start()

	// Register With Generic API
	RegisterAgent(host, addr string, port int) error
	Register(t *Task) error
	DeRegister(id string) error
	AgentHealth(name string) (bool, error)

	// API Cluster Operations
	ListClusters() ([]*Cluster, error)
	GetCluster(id string) (*Cluster, error)
	PutCluster(cluster *Cluster) error
	DelCluster(id string) error

	// API Service Operations
	ListServices() ([]*Service, error)
	GetService(id string) (*Service, error)
	PutService(s *Service) error
	DelService(id string) error

	// API Task Definition Operations
	ListTaskDefinitions() ([]*TaskDefinition, error)
	GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
	PutTaskDefinition(t *TaskDefinition) error

	// API Host Operations
	ListHosts() ([]*Host, error)
	GetHost(id string) (*Host, error)
	PutHost(h *Host) error
	DelHost(id string) error

	// API Task Operations:
	//
	// storing tasks:
	// => state/tasks/<task_id>                      # stores a version of the task by cluster and service
	// => state/hosts/<host_id>/<task_id>            # stores a version of the task by host
	// => state/health/<task_id>                     # marks the task as being healthy or not (unnecessary in consul)
	//
	// Task queries can be executed with a set of options in the TaskQueryOpts, currently
	// tasks can only be queried by using the ByHost or ByService and ByCluster parameters.
	ListTasks(opts *TaskQueryOpts) ([]*Task, error)
	PutTask(t *Task) error
	DelTask(t *Task) error
	TaskHealthy(t *Task) (bool, error)

	// Listen for custom events emitted from the API,
	// can match events using a * pattern.
	// Events that should be emitted on change of a key:
	// => health::task:<node>:<status>:<task_id>
	// => health::host:<status>:<host_id>
	// => config::<resource (service|task_definition|host|cluster)>/<resource_id>
	// => state::<host_id>:<task_id>
	Subscribe(key, evt string, listener chan string)
	UnSubscribe(key string)

	Conf() *StorageConfig
}

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	// this function should start a task.
	StartTask(t *Task) error

	// this function should stop a task from running and is intended to be
	// executed by the agent.
	StopTask(t *Task) error

	// a list of ports that are required by this executor
	ReservedPorts() []uint
}

type Validatable interface {
	Validate(SchedulerApi) []string
}

// encode and decode functions, the encode function will panic if the json marshalling fails.
func encode(item interface{}) []byte {
	res, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	return res
}

func decode(data []byte, item interface{}) {
	err := json.Unmarshal(data, item)
	if err != nil {
		panic(err)
	}
}
