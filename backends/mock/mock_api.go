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
		health:          make(map[string]api.TaskState),
		deployments:     make(map[string]*api.Deployment),
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
	deployments     map[string]*api.Deployment
	tasks           map[string]*api.Task
	hosts           map[string]*api.Host
	locks           map[string]bool
	health          map[string]api.TaskState
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

// API Cluster Operations

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
	a.emit("clusters/" + c.Name)
	return nil
}

func (a *MockApi) DelCluster(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.clusters, id)
	return nil
}

// API Deployment Operations

func (a *MockApi) ListDeployments() ([]*api.Deployment, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	deps := []*api.Deployment{}
	for _, c := range a.deployments {
		deps = append(deps, c)
	}
	return deps, nil
}

func (a *MockApi) GetDeployment(id string) (*api.Deployment, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	c, ok := a.deployments[id]
	if !ok {
		return c, api.ErrNotFound
	}
	return c, nil
}

func (a *MockApi) PutDeployment(c *api.Deployment) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.deployments[c.Name] = c
	a.emit("deploments/" + c.Name)
	return nil
}

func (a *MockApi) DelDeployment(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.deployments, id)
	return nil
}

// API Service Operations

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
	a.emit("services/" + c.Name)
	return nil
}

func (a *MockApi) DelService(id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.services, id)
	return nil
}

// API Task Definition Operations

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
	a.emit("task_defintions/" + c.Name)
	return nil
}

// API Host Operations

func (a *MockApi) ListHosts(opts *api.HostQueryOpts) ([]*api.Host, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	list := []*api.Host{}
	for _, c := range a.hosts {
		list = append(list, c)
	}
	return list, nil
}

func (a *MockApi) GetHost(cluster, id string) (*api.Host, error) {
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
	a.emit("hosts/" + c.Name)
	return nil
}

func (a *MockApi) DelHost(cluster, id string) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.hosts, id)
	return nil
}

// API Task Operations

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
	a.emit("tasks/" + t.Id())
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

func (a *MockApi) ListTasks(q *api.TaskQueryOpts) ([]*api.Task, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	ts := []*api.Task{}

	for _, t := range a.tasks {
		if q.Failing || q.Running {
			state, err := a.GetTaskState(t)
			if err != nil {
				return ts, err
			}

			if q.Failing && state.Healthy() {
				continue
			} else if q.Running && !state.Healthy() {
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

		if q.ByDeployment != "" && q.ByDeployment != t.Deployment {
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

// API Task State Operations

func (a *MockApi) GetTaskState(t *api.Task) (api.TaskState, error) {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.health[t.ID()], nil
}

func (a *MockApi) PutTaskState(taskId string, s api.TaskState) error {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.health[taskId] = s
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
