package api

import "encoding/json"

type TaskState int

const (
	STOPPED TaskState = iota
	RUNNING
	FAILING
)

// From the consul documentation:
// any lockable interface should implement the same functionality.
// Lock attempts to acquire the lock and blocks while doing so.
// Providing a non-nil stopCh can be used to abort the lock attempt.
// Returns a channel that is closed if our lock is lost or an error.
// This channel could be closed at any time due to session invalidation,
// communication errors, operator intervention, etc. It is NOT safe to
// assume that the lock is held until Unlock() unless the Session is specifically
// created without any associated health checks. By default Consul sessions
// prefer liveness over safety and an application must be able to handle
// the lock being lost.
type Lockable interface {
	Lock(stopCh <-chan struct{}) (<-chan struct{}, error)
	Unlock() error
	Destroy() error
}

type StorageConfig struct {
	ConfigPrefix          string
	ClustersPrefix        string
	ServicesPrefix        string
	HostsPrefix           string
	TaskDefinitionsPrefix string
	SchedulersPrefix      string
	StatePrefix           string
}

func DefaultStorageConfig() *StorageConfig {
	confPrefix := "config/"

	return &StorageConfig{
		ConfigPrefix:          confPrefix,
		ClustersPrefix:        confPrefix + "clusters/",
		ServicesPrefix:        confPrefix + "service/",
		HostsPrefix:           confPrefix + "hosts/",
		TaskDefinitionsPrefix: confPrefix + "task_definitions/",
		SchedulersPrefix:      "schedulers/",
		StatePrefix:           "state/",
	}
}

type TaskQueryOpts struct {
	ByService string
	ByHost    string
	ByState   bool
	State     TaskState
}

type SchedulerApi interface {

	Lock(key string) (Lockable, error)

	// Register With Generic API
	Register(t *Task) error
	DeRegister(id string) error

	// API Cluster Operations
	ListClusters() ([]*Cluster, error)
	GetCluster(id string) (*Cluster, error)
	PutCluster(cluster *Cluster) error
	DelCluster(id string) error

	// API Service Operations
	ListServices() ([]*Service, error)
	GetService(id string) (*Service, error)
	PutService(s *Service) (*Service, error)
	DelService(id string) (*Service, error)

	// API Task Definition Operations
	ListTaskDefinitions() ([]*TaskDefinition, error)
	GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
	PutTaskDefinition(t *TaskDefinition) error

	// API Host Operations
	ListHosts() ([]*Host, error)
	GetHost(id string) (*Host, error)
	PutHost(h *Host) (error)
	DelHost(id string) (error)

	// API Task Operations
	ListTasks(q *TaskQueryOpts) ([]*Task, error)
	GetTask(id string) ([]*Task, error)
	ScheduleTask(task *Task) error
	DeScheduleTask(task *Task) error

	// Listen for custom events emitted from the API,
	// can match events using a * pattern.
	// Events that should be emitted on change:
	// health:<status (failing|passing)>:<task_id>
	// config:<resource (service|task_definition|host|cluster)>_<resource_id>
	Listen(evt string, listener chan string)
}

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	StartTask(t Task) error
	StopTask(t Task) error
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
	json.Unmarshal(data, item)
}
