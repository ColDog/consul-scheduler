package api

import (
	"errors"
	"github.com/coldog/sked/api/state"
)

var (
	ErrTxFailed = errors.New("consul transaction has failed")
	ErrNotFound = errors.New("could not find the requested resource")
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

//type ServiceQueryOpts struct  {
//	ByStatus TaskState
//	ByVersion int
//	ByHost int
//}

type SchedulerApi interface {

	// Certain backends, ie. Consul, should know the hostname to give to the agent.
	HostName() (string, error)

	// Start should perform the necessary setup tasks for the api.
	Start()

	// acquire a lock on a resource from consul, does not block when the lock cannot be held, rather
	// it should return immediately
	Lock(key string, block bool) (Lockable, error)

	// Health checks for agents. These are implemented as simple TTL on a key, if the key does not exist, or is
	// expired (ttl is 30 seconds) then the agent is marked as unhealthy.
	GetAgentHealth(name string) (bool, error)
	SetAgentHealthy(name string) error

	// API Cluster Operations
	ListClusters() ([]*Cluster, error)
	GetCluster(id string) (*Cluster, error)
	PutCluster(cluster *Cluster) error
	DelCluster(id string) error

	// API Service Operations
	ListDeployments() ([]*Deployment, error)
	GetDeployment(id string) (*Deployment, error)
	PutDeployment(s *Deployment) error
	DelDeployment(id string) error

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
	// storing tasks:
	// => state/tasks/<task_id>                      # stores a version of the task by cluster and service
	// => state/hosts/<host_id>/<task_id>            # stores a version of the task by host
	// Task queries can be executed with a set of options in the TaskQueryOpts, currently
	// tasks can only be queried by using the ByHost or ByService and ByCluster parameters.
	ListTasks(opts *TaskQueryOpts) ([]*Task, error)
	PutTask(t *Task) error
	DelTask(t *Task) error

	// Task health operations. These are implemented in different ways for each backend
	// if consul has any health checks these will be used, otherwise the default kv check
	// will be consulted.
	// storage:
	// => state/health/<task_id>                     # marks the task as being healthy or not
	GetTaskState(taskId string) (TaskState, error)
	PutTaskState(taskId string, s TaskState) error

	// Listen for custom events emitted from the API,
	// can match events using a * pattern.
	// Events that should be emitted on change of a key:
	// => health::task:<node>:<status>:<task_id>
	// => health::host:<status>:<host_id>
	// => config::<resource (service|task_definition|host|cluster)>/<resource_id>
	// => state::<host_id>:<task_id>
	Subscribe(key, evt string, listener chan string)
	UnSubscribe(key string)

	// Service Discovery idea:
	// a service is a mapping of a cluster, deployment, container and port. This allows the engine to return the
	// list of running
	// ListServices() ([]*Service, error)

	// Get the configuration for this storage
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

	// attempts to check whether a task is running from the executor level.
	// if the executor cannot answer or is unsure it should return an error.
	IsRunning() (bool, error)
}

type Validatable interface {
	Validate(SchedulerApi) []string
}
