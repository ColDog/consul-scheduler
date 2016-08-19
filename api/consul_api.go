package api

import (
	"errors"

	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"
	"sync"
)

var (
	ErrTxFailed = errors.New("consul transaction has failed")
	ErrNotFound = errors.New("could not find the requested resource")
)

type listener struct {
	on string
	ch chan string

}

type ConsulApi struct {
	kv         *api.KV
	agent      *api.Agent
	catalog    *api.Catalog
	health     *api.Health
	client     *api.Client
	ConsulConf *api.Config
	conf       *StorageConfig
	listeners  map[string]*listener
	eventLock  *sync.RWMutex
}

func NewConsulApi(conf *StorageConfig) *ConsulApi {
	a := newConsulApi()
	a.conf = conf

	go a.monitorConfig()
	go a.monitorHealth()

	return a
}

func newConsulApi() *ConsulApi {
	apiConfig := api.DefaultConfig()

	client, err := api.NewClient(apiConfig)
	if err != nil {
		log.Fatal(err)
	}

	return &ConsulApi{
		kv:         client.KV(),
		agent:      client.Agent(),
		catalog:    client.Catalog(),
		health:     client.Health(),
		listeners:  make(map[string]*listener),
		eventLock:  &sync.RWMutex{},
		client:     client,
		ConsulConf: apiConfig,
		conf:       DefaultStorageConfig(),
	}
}

func (a *ConsulApi) HostName() (string, error) {
	return a.agent.NodeName()
}

func (a *ConsulApi) Conf() *StorageConfig {
	return a.conf
}

func (a *ConsulApi) put(key string, value []byte, flags ...uint64) error {
	flag := uint64(0)
	if len(flags) >= 1 {
		flag = flags[0]
	}

	_, err := a.kv.Put(&api.KVPair{Key: key, Value: value, Flags: flag}, nil)
	if err != nil {
		log.WithField("consul-api", "put").WithField("key", key).Error(err)
	}
	return err
}

func (a *ConsulApi) del(key string) error {
	_, err := a.kv.Delete(key, nil)
	if err != nil {
		log.WithField("consul-api", "del").WithField("key", key).Error(err)
	}
	return err
}

func (a *ConsulApi) get(key string) (*api.KVPair, error) {
	res, _, err := a.kv.Get(key, nil)
	if err != nil {
		log.WithField("consul-api", "get").WithField("key", key).Error(err)
		return res, ErrNotFound
	}

	if res == nil || res.Value == nil {
		return res, ErrNotFound
	}

	return res, err
}

func (a *ConsulApi) list(prefix string) (api.KVPairs, error) {
	res, _, err := a.kv.List(prefix, nil)
	if err != nil {
		log.WithField("consul-api", "put").WithField("key", prefix).Error(err)
	}
	return res, err
}

func (a *ConsulApi) Lock(key string) (Lockable, error) {
	return a.client.LockKey(key)
}

// ==> REGISTER & DEREGISTER

func (a *ConsulApi) RegisterAgent(host, addr string, port int) error {
	return a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID: "consul-scheduler-"+host,
		Name: "consul-scheduler",
		Port: port,
		Address: addr,
		Checks: api.AgentServiceChecks{
			&api.AgentServiceCheck{
				Interval: "30s",
				HTTP: fmt.Sprintf("http://%s:%d", addr, port),
			},
		},
	})
}

func (a *ConsulApi) Register(t *Task) error {

	checks := api.AgentServiceChecks{}
	for _, check := range t.TaskDefinition.Checks {
		checks = append(checks, &api.AgentServiceCheck{
			Interval: check.Interval,
			Script:   check.Script,
			Timeout:  check.Timeout,
			TCP:      check.TCP,
			HTTP:     check.HTTP,
		})
	}

	log.WithField("id", t.Id()).WithField("name", t.Name()).WithField("host", t.Host).Debug("registering task")

	return a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID:      t.Id(),
		Name:    t.Name(),
		Tags:    append(t.TaskDefinition.Tags, t.Cluster.Name, t.Service),
		Port:    int(t.Port),
		Address: t.Host,
		Checks:  checks,
	})
}

func (a *ConsulApi) DeRegister(taskId string) error {
	return a.agent.ServiceDeregister(taskId)
}

// ==> CLUSTER operations

func (a *ConsulApi) ListClusters() (clusters []*Cluster, err error) {
	list, err := a.list(a.conf.ClustersPrefix)
	if err != nil {
		return clusters, err
	}

	for _, v := range list {
		c := &Cluster{}
		decode(v.Value, c)
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func (a *ConsulApi) GetCluster(id string) (*Cluster, error) {
	c := &Cluster{}

	kv, err := a.get(a.conf.ClustersPrefix + id)
	if kv == nil || kv.Value == nil {
		return c, ErrNotFound
	}

	if err == nil {
		decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutService(s *Service) error {
	return a.put(a.conf.ServicesPrefix+s.Name, encode(s))
}

func (a *ConsulApi) DelService(id string) error {
	return a.del(a.conf.ServicesPrefix + id)
}

// ==> SERVICE operations

func (a *ConsulApi) ListServices() (services []*Service, err error) {
	list, err := a.list(a.conf.ServicesPrefix)
	if err != nil {
		return services, err
	}

	for _, v := range list {
		c := &Service{}
		decode(v.Value, c)
		services = append(services, c)
	}
	return services, nil
}

func (a *ConsulApi) GetService(id string) (*Service, error) {
	c := &Service{}

	kv, err := a.get(a.conf.ServicesPrefix + id)
	if kv == nil || kv.Value == nil {
		return c, ErrNotFound
	}

	if err == nil {
		decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutCluster(c *Cluster) error {
	return a.put(a.conf.ClustersPrefix+c.Name, encode(c))
}

func (a *ConsulApi) DelCluster(id string) error {
	return a.del(a.conf.ClustersPrefix + id)
}

// ==> HOST operations

func (a *ConsulApi) ListHosts() (hosts []*Host, err error) {
	list, err := a.list(a.conf.HostsPrefix)
	if err != nil {
		return hosts, err
	}

	for _, v := range list {
		h := &Host{}
		decode(v.Value, h)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func (a *ConsulApi) GetHost(name string) (*Host, error) {
	h := &Host{}

	kv, err := a.get(a.conf.HostsPrefix + name)
	if kv == nil || kv.Value == nil {
		return h, ErrNotFound
	}

	if err == nil {
		decode(kv.Value, h)
	}

	return h, err
}

func (a *ConsulApi) PutHost(h *Host) error {
	return a.put(a.conf.HostsPrefix+h.Name, encode(h))
}

func (a *ConsulApi) DelHost(name string) error {
	return a.del(a.conf.HostsPrefix + name)
}

// ==> TASK DEFINITION operations

func (a *ConsulApi) ListTaskDefinitions() (taskDefs []*TaskDefinition, err error) {
	list, err := a.list(a.conf.TaskDefinitionsPrefix)
	if err != nil {
		return taskDefs, err
	}

	for _, v := range list {
		c := &TaskDefinition{}
		decode(v.Value, c)
		taskDefs = append(taskDefs, c)
	}
	return taskDefs, nil
}

func (a *ConsulApi) GetTaskDefinition(name string, version uint) (*TaskDefinition, error) {
	t := &TaskDefinition{}
	id := fmt.Sprintf("%s%s/%d", a.conf.TaskDefinitionsPrefix, name, version)

	kv, err := a.get(id)
	if kv == nil || kv.Value == nil {
		return t, ErrNotFound
	}

	if err == nil {
		decode(kv.Value, t)
	}

	return t, err
}

func (a *ConsulApi) PutTaskDefinition(t *TaskDefinition) error {
	id := fmt.Sprintf("%s%s/%d", a.conf.TaskDefinitionsPrefix, t.Name, t.Version)
	return a.put(id, encode(t))
}

// ==> TASK operations
// tasks are stored under the following keys
// > /state-prefix/<host-id>/<task-id>
// > /state-prefix/<task-id>
//
// When a task is deleted, it is removed from the first key, but kept under the second key
// with the 'scheduled' attribute set to false.

func (a *ConsulApi) ListTasks(q *TaskQueryOpts) (ts []*Task, err error) {
	prefix := a.conf.StatePrefix

	// add the host or service prefix to the task, which will scope in what is returned
	if q.ByHost != "" {
		prefix += q.ByHost
	}

	if q.ByService != "" {
		prefix += q.ByService
	}

	list, err := a.list(prefix)
	if err != nil {
		return ts, err
	}

	for _, v := range list {
		t := &Task{}
		decode(v.Value, t)

		t.Passing = a.taskStatus(t)

		// handle the by state queries here.
		if q.Failing && t.Passing {
			continue
		}

		if q.Running && !t.Passing {
			continue
		}

		if q.Scheduled && !t.Scheduled {
			continue
		}

		ts = append(ts, t)
	}
	return ts, nil
}

func (a *ConsulApi) GetTask(id string) (*Task, error) {
	t := &Task{}

	kv, err := a.get(a.conf.TaskDefinitionsPrefix + id)
	if err == nil {
		decode(kv.Value, t)
		t.Passing = a.taskStatus(t)
	}

	return t, err
}

func (a *ConsulApi) ScheduleTask(t *Task) error {
	body := encode(t)

	// ensure this is set to true before serializing
	t.Scheduled = true
	ops := api.KVTxnOps{
		&api.KVTxnOp{
			Verb:  "set",
			Key:   a.conf.StatePrefix + t.Host + "/" + t.Id(),
			Value: body,
		},
		&api.KVTxnOp{
			Verb:  "set",
			Key:   a.conf.StatePrefix + t.Id(),
			Value: body,
		},
	}

	ok, _, _, err := a.kv.Txn(ops, nil)
	if !ok {
		return ErrTxFailed
	}

	if err != nil {
		return err
	}

	return nil
}

func (a *ConsulApi) DeScheduleTask(t *Task) error {

	// setting scheduled = false means this task won't be picked up by the query as running
	t.Scheduled = false
	ops := api.KVTxnOps{
		&api.KVTxnOp{
			Verb: "delete",
			Key:  a.conf.StatePrefix + t.Host + "/" + t.Id(),
		},
		&api.KVTxnOp{
			Verb:  "set",
			Key:   a.conf.StatePrefix + t.Id(),
			Value: encode(t),
		},
	}

	ok, _, _, err := a.kv.Txn(ops, nil)
	if !ok {
		return ErrTxFailed
	}

	if err != nil {
		return err
	}

	return nil
}

// used to find out if a task is passing.
func (a *ConsulApi) taskStatus(t *Task) bool {
	s, _, err := a.health.Checks(t.Name(), nil)
	if err != nil {
		return false
	}

	for _, ch := range s {
		if ch.ServiceID == t.Id() {
			return ch.Status == "passing"
		}
	}

	return false
}
