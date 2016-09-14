package api

import (
	"errors"
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

type TaskQueryOpts struct {
	ByCluster string
	ByDeployment string
	ByHost    string
	Running   bool
	Scheduled bool
	Failing   bool
	Rejected  bool
}

type ServiceQueryOpts struct {
	ByStatus  TaskState
	ByVersion int
	ByHost    int
}

type HostQueryOpts struct {
	ByCluster string
}

type EndpointQuery struct {
	ByService string
}

type SchedulerApi interface {

	// Certain backends, ie. Consul, should know the hostname to give to the agent.
	HostName() (string, error)

	// Start should perform the necessary setup tasks for the api.
	Start()

	// acquire a lock on a resource from consul, does not block when the lock cannot be held, rather
	// it should return immediately
	Lock(key string, block bool) (Lockable, error)

	// API Cluster Operations
	// => config/clusters/<cluster_id>
	ListClusters() ([]*Cluster, error)
	GetCluster(id string) (*Cluster, error)
	PutCluster(cluster *Cluster) error
	DelCluster(id string) error

	// API Deployment Operations
	// => config/deployments/<deployment_id>
	ListDeployments() ([]*Deployment, error)
	GetDeployment(id string) (*Deployment, error)
	PutDeployment(s *Deployment) error
	DelDeployment(id string) error

	// API Task Definition Operations
	// => config/task_definitions/<task_definition_id>/<version>
	ListTaskDefinitions() ([]*TaskDefinition, error)
	GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
	PutTaskDefinition(t *TaskDefinition) error

	// API Service Operations
	// storage:
	// => config/services/<service_id>
	ListServices() ([]*Service, error)
	PutService(s *Service) error
	GetService(id string) (*Service, error)
	DelService(id string) error

	// API Host Operations
	// Host should have a TTL on it to implement health checking.
	// => hosts/<cluster>/<host_id>
	ListHosts(opts *HostQueryOpts) ([]*Host, error)
	GetHost(cluster, id string) (*Host, error)
	PutHost(h *Host) error
	DelHost(cluster, id string) error

	// API Task Operations:
	// storing tasks:
	// => state/tasks/<task_id> # stores a version of the task by cluster and service
	// Task queries can be executed with a set of options in the TaskQueryOpts, currently
	// tasks can only be queried by using the ByHost or ByService and ByCluster parameters.
	ListTasks(opts *TaskQueryOpts) ([]*Task, error)
	PutTask(t *Task) error
	DelTask(t *Task) error

	// Task health operations. These are implemented in different ways for each backend
	// if consul has any health checks these will be used, otherwise the default kv check
	// will be consulted.
	// => state/health/<task_id>
	GetTaskState(t *Task) (TaskState, error)
	PutTaskState(taskId string, s TaskState) error

	// Listen for custom events emitted from the API,
	// can match events using a * pattern.
	// Events that should be emitted on change of a key:
	// => health::task:<node>:<status>:<task_id>
	// => health::node:<status>:<node_id>
	// => hosts::<host_id>
	// => config::<resource (service|task_definition|host|cluster)>/<resource_id>
	// => state::<host_id>:<task_id>
	Subscribe(key, evt string, listener chan string)
	UnSubscribe(key string)
}

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	// this function should start a task.
	StartTask(t *Task, cont *Container) error

	// this function should stop a task from running and is intended to be
	// executed by the agent.
	StopTask(t *Task, cont *Container) error

	// attempts to check whether a task is running from the executor level.
	// if the executor cannot answer or is unsure it should return an error.
	IsRunning(t *Task, cont *Container) (bool, error)
}

type Validatable interface {
	Validate(SchedulerApi) []string
}
