package api

import (
	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"
)

var (
	ErrTxFailed = errors.New("consul transaction has failed")
)

type ConsulApi struct {
	kv         *api.KV
	agent      *api.Agent
	catalog    *api.Catalog
	health     *api.Health
	client     *api.Client
	ConsulConf *api.Config
	conf       *StorageConfig
}

func NewConsulApi(conf *StorageConfig) *ConsulApi {
	apiConfig := api.DefaultConfig()

	client, err := api.NewClient(apiConfig)
	if err != nil {
		log.Fatal(err)
	}

	a := &SchedulerApi{
		kv:         client.KV(),
		agent:      client.Agent(),
		catalog:    client.Catalog(),
		health:     client.Health(),
		client:     client,
		ConsulConf: apiConfig,
		conf:       conf,
	}

	return a
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

// ==> REGISTER & DEREGISTER

func (a *ConsulApi) Register(t *Task) error {

	checks := api.AgentServiceChecks{}
	for _, check := range t.TaskDef.Checks {
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
		Tags:    append(t.TaskDef.Tags, t.Cluster.Name, t.Service),
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

func (a *ConsulApi) GetTaskDefinition(id string) (*Service, error) {
	c := &Service{}

	kv, err := a.get(a.conf.TaskDefinitionsPrefix + id)
	if err == nil {
		decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutTaskDefinition(c *TaskDefinition) error {
	return a.put(a.conf.TaskDefinitionsPrefix+c.Name, encode(c))
}

func (a *ConsulApi) DelTaskDefinition(id string) error {
	return a.del(a.conf.TaskDefinitionsPrefix + id)
}

// ==> TASK operations
// tasks are stored under the following keys
// > /state-prefix/<host-id>/<task-id>
// > /state-prefix/<task-id>
//
// When a task is deleted, it is removed from the first key, but kept under the second key
// with the 'scheduled' attribute set to false.

func (a *ConsulApi) ListTasks(q *TaskQueryOpts) (taskDefs []*TaskDefinition, err error) {
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
		return taskDefs, err
	}

	for _, v := range list {
		t := &Task{}
		decode(v.Value, t)

		t.Passing = a.taskStatus(t)

		// handle the by state queries here.
		if q.ByState {
			if q.State == RUNNING {
				if !t.Passing {
					continue
				}

			} else if q.State == FAILING {
				if t.Passing {
					continue
				}

			} else if q.State == STOPPED {
				if t.Scheduled {
					continue
				}
			}
		}

		taskDefs = append(taskDefs, t)
	}
	return taskDefs, nil
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
		return false, err
	}

	for _, ch := range s {
		if ch.ServiceID == t.Id() {
			return ch.Status == "passing"
		}
	}

	return false
}
