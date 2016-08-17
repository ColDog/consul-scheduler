package api

type TaskState int

const (
	STOPPED TaskState = iota
	RUNNING
	FAILING
)

type StorageConfig struct {
	ConfigPrefix string
	ClustersPrefix string
	ServicesPrefix string
	HostsPrefix string
	TaskDefinitionsPrefix string
	SchedulersPrefix string
	StatePrefix string
}

func DefaultStorageConfig() {
	confPrefix := "config/"

	return &StorageConfig{
		ConfigPrefix: confPrefix,
		ClustersPrefix: confPrefix+"clusters/",
		ServicesPrefix: confPrefix+"service/",
		HostsPrefix: confPrefix+"hosts/",
		TaskDefinitionsPrefix: confPrefix+"task_definitions/",
		SchedulersPrefix: "schedulers/",
		StatePrefix: "state/",
	}
}

type TaskQueryOpts struct {
	ByService string
	ByHost    string
	ByState   bool
	State     TaskState
}

type GenericApi interface {

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
	GetTaskDefinition(id string) (*TaskDefinition, error)
	PutTaskDefinition(t *TaskDefinition) error
	DelTaskDefinition(id string) error

	// API Task Operations
	ListTasks(q *TaskQueryOpts) ([]*Task, error)
	GetTask(id string) ([]*Task, error)
	ScheduleTask(task *Task) error
	DeScheduleTask(task *Task) error
	TaskStatus(id string) (string, error)

	// Listen for custom events emitted from the API,
	// can match events using a * pattern.
	Listen(evt string, chan struct{})
}
