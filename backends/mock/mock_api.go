package mock

import (
	"sync"
	"github.com/coldog/sked/api"
)

func NewMockApi() *MockApi {
	return &MockApi{
		clusters:        make(map[string]*api.Cluster),
		services:        make(map[string]*api.Service),
		taskDefinitions: make(map[string]*api.TaskDefinition),
		tasks:           make(map[string]*api.Task),
		hosts:           make(map[string]*api.Host),
		health:          make(map[string]string),
		listeners: make(map[string]struct {
			on string
			ch chan string
		}),
		locks: make(map[string]bool),
		lock:  &sync.RWMutex{},
	}
}

type MockApi struct {
	clusters        map[string]*api.Cluster
	services        map[string]*api.Service
	taskDefinitions map[string]*api.TaskDefinition
	tasks           map[string]*api.Task
	hosts           map[string]*api.Host
	locks           map[string]bool
	health          map[string]string
	listeners       map[string]struct {
		on string
		ch chan string
	}
	lock *sync.RWMutex
}

func (a *MockApi) HostName() (string, error) {
	return "test", nil
}

func (a *MockApi) Lock(key string, block bool) (api.Lockable, error) {
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

func (a *MockApi) Register(t *api.Task) error {
	return nil
}

func (a *MockApi) DeRegister(id string) error {
	return nil
}

func (a *MockApi) AgentHealth(name string) (bool, error) {
	return true, nil
}

func (a *MockApi) ListClusters() ([]*api.Cluster, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	clus := []*api.Cluster{}
	for _, c := range a.clusters {
		clus = append(clus, c)
	}
	return clus, nil
}

func (a *MockApi) GetCluster(id string) (*api.Cluster, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.clusters[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutCluster(c *api.Cluster) error {
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

func (a *MockApi) ListServices() ([]*api.Service, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*api.Service{}
	for _, c := range a.services {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetService(id string) (*api.Service, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.services[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutService(c *api.Service) error {
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

func (a *MockApi) ListTaskDefinitions() ([]*api.TaskDefinition, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*api.TaskDefinition{}
	for _, c := range a.taskDefinitions {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetTaskDefinition(id string, ver uint) (*api.TaskDefinition, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.taskDefinitions[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutTaskDefinition(c *api.TaskDefinition) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.taskDefinitions[c.Name] = c
	a.emit(a.Conf().TaskDefinitionsPrefix + c.Name)
	return nil
}

func (a *MockApi) ListHosts() ([]*api.Host, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*api.Host{}
	for _, c := range a.hosts {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetHost(id string) (*api.Host, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.hosts[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutHost(c *api.Host) error {
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

func (a *MockApi) GetTask(id string) (*api.Task, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.tasks[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutTask(t *api.Task) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.tasks[t.Id()] = t
	a.emit(a.Conf().TasksPrefix + t.Id())
	return nil
}

func (a *MockApi) DelTask(t *api.Task) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	if _, ok := a.tasks[t.Id()]; ok {
		delete(a.tasks, t.Id())
	}

	return nil
}

func (a *MockApi) TaskHealthy(t *api.Task) (bool, error) {
	return true, nil
}

func (a *MockApi) ListTasks(q *api.TaskQueryOpts) ([]*api.Task, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	ts := []*api.Task{}

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

func (a *MockApi) Conf() *api.StorageConfig {
	return api.DefaultStorageConfig()
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

func (a *MockApi) PutTaskHealth(taskId, status string) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.health[taskId] = status
	return nil
}

func match(key, evt string) bool {
	evtR := []rune(evt)
	for idx, char := range key {
		if char == '*' {
			return true
		}

		if idx > (len(evtR) - 1) {
			return false
		}

		if char != evtR[idx] {
			return false
		}
	}

	return true
}
