package api

import "sync"

func NewMockApi() *MockApi {
	return &MockApi{
		clusters:        make(map[string]*Cluster),
		services:        make(map[string]*Service),
		taskDefinitions: make(map[string]*TaskDefinition),
		tasks:           make(map[string]*Task),
		hosts:           make(map[string]*Host),
		listeners: make(map[string]struct {
			on string
			ch chan string
		}),
		locks: make(map[string]bool),
		lock:  &sync.RWMutex{},
	}
}

type MockApi struct {
	clusters        map[string]*Cluster
	services        map[string]*Service
	taskDefinitions map[string]*TaskDefinition
	tasks           map[string]*Task
	hosts           map[string]*Host
	locks           map[string]bool
	listeners       map[string]struct {
		on string
		ch chan string
	}
	lock *sync.RWMutex
}

func (a *MockApi) HostName() (string, error) {
	return "test", nil
}

func (a *MockApi) Lock(key string, block bool) (Lockable, error) {
	return NewMockLock(key, a), nil
}

func (a *MockApi) Wait() error {
	return nil
}

func (a *MockApi) Start() {

}

func (a *MockApi) RegisterAgent(host, addr string) error {
	return nil
}

func (a *MockApi) DeRegisterAgent(id string) error {
	return nil
}

func (a *MockApi) Register(t *Task) error {
	return nil
}

func (a *MockApi) DeRegister(id string) error {
	return nil
}

func (a *MockApi) AgentHealth(name string) (bool, error) {
	return true, nil
}

func (a *MockApi) ListClusters() ([]*Cluster, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	clus := []*Cluster{}
	for _, c := range a.clusters {
		clus = append(clus, c)
	}
	return clus, nil
}

func (a *MockApi) GetCluster(id string) (*Cluster, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.clusters[id]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutCluster(c *Cluster) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.clusters[c.Name] = c
	a.emit(a.Conf().ClustersPrefix + c.Name)
	return nil
}

func (a *MockApi) DelCluster(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.clusters, id)
	return nil
}

func (a *MockApi) ListServices() ([]*Service, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*Service{}
	for _, c := range a.services {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetService(id string) (*Service, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.services[id]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutService(c *Service) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.services[c.Name] = c
	a.emit(a.Conf().ServicesPrefix + c.Name)
	return nil
}

func (a *MockApi) DelService(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.services, id)
	return nil
}

func (a *MockApi) ListTaskDefinitions() ([]*TaskDefinition, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*TaskDefinition{}
	for _, c := range a.taskDefinitions {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetTaskDefinition(id string, ver uint) (*TaskDefinition, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.taskDefinitions[id]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutTaskDefinition(c *TaskDefinition) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.taskDefinitions[c.Name] = c
	a.emit(a.Conf().TaskDefinitionsPrefix + c.Name)
	return nil
}

func (a *MockApi) ListHosts() ([]*Host, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*Host{}
	for _, c := range a.hosts {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetHost(id string) (*Host, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.hosts[id]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutHost(c *Host) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.hosts[c.Name] = c
	a.emit(a.Conf().HostsPrefix + c.Name)
	return nil
}

func (a *MockApi) DelHost(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.hosts, id)
	return nil
}

// API Task Operations:
//
// storing tasks:
// => state/tasks/<cluster>/<service>/<task_id>  # stores a version of the task by cluster and service
// => state/hosts/<host_id>/<task_id>            # stores a version of the task by host
// => state/scheduling/<task_id>                 # marks the task as being scheduled or not scheduled
// => state/rejections/<task_id>                 # marks the task as being rejected by the agent
// => state/health/<task_id>                     # marks the task as being healthy or not (unnecessary in consul)
//
// Task queries can be executed with a set of options in the TaskQueryOpts, currently
// tasks can only be queried by using the ByHost or ByService and ByCluster parameters.
//ListTasks(opts *TaskQueryOpts) ([]*Task, error)
//
//GetTask(id string) (*Task, error)
//ScheduleTask(t *Task) error
//DeScheduleTask(t *Task) error
//DelTask(t *Task) error
//RejectTask(t *Task, reason string) error
//
//TaskHealthy(t *Task) (bool, error)
//TaskScheduled(t *Task) (bool, error)
//TaskRejected(t *Task) (bool, error)

func (a *MockApi) GetTask(id string) (*Task, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.tasks[id]
	if !ok {
		return c, ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutTask(t *Task) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.tasks[t.Id()] = t
	a.emit(a.Conf().TasksPrefix + t.Id())
	return nil
}

func (a *MockApi) DelTask(t *Task) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if _, ok := a.tasks[t.Id()]; ok {
		delete(a.tasks, t.Id())
	}

	return nil
}

func (a *MockApi) TaskHealthy(t *Task) (bool, error) {
	return true, nil
}

func (a *MockApi) ListTasks(q *TaskQueryOpts) ([]*Task, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	ts := []*Task{}

	for _, t := range a.tasks {
		if q.Failing || q.Running {
			health, err := a.TaskHealthy(t)
			if err != nil {
				return ts, err
			}

			if q.Failing && health {
				continue
			} else if q.Running && !health {
				continue
			}
		}

		if q.Scheduled && !t.Scheduled {
			continue
		}

		if q.Rejected && !t.Rejected {
			continue
		}

		if q.ByHost != "" && t.Host != q.ByHost {
			continue
		}

		if q.ByCluster != "" && q.ByCluster != t.Cluster {
			continue
		}

		if q.ByService != "" && q.ByService != t.Service {
			continue
		}

		t.api = a
		ts = append(ts, t)
	}

	return ts, nil
}

func (a *MockApi) Subscribe(key, evt string, listener chan string) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.listeners[key] = struct {
		on string
		ch chan string
	}{evt, listener}
}

func (a *MockApi) UnSubscribe(id string) {

}

func (a *MockApi) Conf() *StorageConfig {
	return DefaultStorageConfig()
}

func (a *MockApi) emit(evt string) {
	evt = "config::" + evt

	for _, l := range a.listeners {
		if match(l.on, evt) {
			select {
			case l.ch <- evt:
			default:
			}
		}
	}
}
