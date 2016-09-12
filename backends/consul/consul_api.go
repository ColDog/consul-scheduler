package consul

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/backends"

	consul "github.com/hashicorp/consul/api"
	log "github.com/Sirupsen/logrus"

	"fmt"
	"sync"
	"time"
)

type listener struct {
	on string
	ch chan string
}

type ConsulApi struct {
	kv         *consul.KV
	agent      *consul.Agent
	catalog    *consul.Catalog
	health     *consul.Health
	client     *consul.Client
	ConsulConf *consul.Config
	prefix     string
	listeners  map[string]*listener
	eventLock  *sync.RWMutex
	registered bool
}

func NewConsulApi(prefix string, apiConf *consul.Config) *ConsulApi {
	client, err := consul.NewClient(apiConf)
	if err != nil {
		log.Fatal(err)
	}

	a := &ConsulApi{
		kv:        client.KV(),
		agent:     client.Agent(),
		catalog:   client.Catalog(),
		health:    client.Health(),
		listeners: make(map[string]*listener),
		eventLock: &sync.RWMutex{},
		client:    client,
	}

	return a
}

func newConsulApi() *ConsulApi {
	apiConfig := consul.DefaultConfig()

	client, err := consul.NewClient(apiConfig)
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
	}
}

func (a *ConsulApi) Start() {
	for {
		_, _, err := a.kv.Get("test", nil)
		if err == nil {
			log.Warn("[consul-api] waiting for consul...")
			break
		}

		time.Sleep(5 * time.Second)
	}

	go a.monitor(a.prefix + "/config/task_definitions", "config")
	go a.monitor(a.prefix + "/config/deployments", "config")
	go a.monitor(a.prefix + "/config/services", "config")
	go a.monitor(a.prefix + "/config/clusters", "config")
	go a.monitor(a.prefix + "/state", "state")
	go a.monitorHealth()
}

func (a *ConsulApi) HostName() (string, error) {
	return a.agent.NodeName()
}

func (a *ConsulApi) put(key string, value []byte, flags ...uint64) error {
	flag := uint64(0)
	if len(flags) >= 1 {
		flag = flags[0]
	}

	_, err := a.kv.Put(&consul.KVPair{Key: a.prefix+"/"+key, Value: value, Flags: flag}, nil)
	if err != nil {
		log.WithField("consul-api", "put").WithField("key", key).Error(err)
	}
	return err
}

func (a *ConsulApi) del(key string) error {
	_, err := a.kv.Delete(a.prefix+"/"+key, nil)
	if err != nil {
		log.WithField("consul-api", "del").WithField("key", key).Error(err)
	}
	return err
}

func (a *ConsulApi) get(key string) (*consul.KVPair, error) {
	res, _, err := a.kv.Get(a.prefix + "/" + key, nil)
	if err != nil {
		log.WithField("consul-api", "get").WithField("key", key).Error(err)
		return res, api.ErrNotFound
	}

	if res == nil || res.Value == nil {
		return res, api.ErrNotFound
	}

	return res, err
}

func (a *ConsulApi) list(prefix string) (consul.KVPairs, error) {
	res, _, err := a.kv.List(a.prefix + "/" + prefix, nil)
	if err != nil {
		log.WithField("consul-api", "put").WithField("key", prefix).Error(err)
	}
	return res, err
}

func (a *ConsulApi) Lock(key string, block bool) (api.Lockable, error) {
	l, err := a.client.LockOpts(&consul.LockOptions{
		Key:         key,
		LockTryOnce: !block,
	})

	lock := &ConsulLockWrapper{lock: l}
	return lock, err
}

// ==> REGISTER & DEREGISTER



// ==> CLUSTER operations

func (a *ConsulApi) ListClusters() (clusters []*api.Cluster, err error) {
	list, err := a.list("config/clusters")
	if err != nil {
		return clusters, err
	}

	for _, v := range list {
		c := &api.Cluster{}
		backends.Decode(v.Value, c)
		clusters = append(clusters, c)
	}
	return clusters, nil
}

func (a *ConsulApi) GetCluster(id string) (*api.Cluster, error) {
	c := &api.Cluster{}

	kv, err := a.get("config/clusters/" + id)
	if kv == nil || kv.Value == nil {
		return c, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutCluster(c *api.Cluster) error {
	return a.put("config/clusters/" + c.Name, backends.Encode(c))
}

func (a *ConsulApi) DelCluster(id string) error {
	return a.del("config/clusters/" + id)
}



// ==> SERVICE operations

func (a *ConsulApi) PutService(s *api.Service) error {
	return a.put("config/services/"+s.Name, backends.Encode(s))
}

func (a *ConsulApi) DelService(id string) error {
	return a.del("config/services/" + id)
}

func (a *ConsulApi) ListServices() (services []*api.Service, err error) {
	list, err := a.list("config/services")
	if err != nil {
		return services, err
	}

	for _, v := range list {
		c := &api.Service{}
		backends.Decode(v.Value, c)
		services = append(services, c)
	}
	return services, nil
}

func (a *ConsulApi) GetService(id string) (*api.Service, error) {
	c := &api.Service{}

	kv, err := a.get("config/services/" + id)
	if kv == nil || kv.Value == nil {
		return c, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, c)
	}

	return c, err
}




// ==> HOST operations

func (a *ConsulApi) ListHosts(opts *api.HostQueryOpts) (hosts []*api.Host, err error) {
	list, err := a.list("hosts/" + opts.ByCluster)
	if err != nil {
		return hosts, err
	}

	for _, v := range list {
		h := &api.Host{}
		backends.Decode(v.Value, h)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func (a *ConsulApi) GetHost(cluster, name string) (*api.Host, error) {
	h := &api.Host{}

	kv, err := a.get("hosts/" + name)
	if kv == nil || kv.Value == nil {
		return h, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, h)
	}

	return h, err
}

func (a *ConsulApi) PutHost(h *api.Host) error {
	err := a.put("hosts/"+h.Cluster+"/"+h.Name, backends.Encode(h))
	if err != nil {
		return err
	}

	if a.registered {
		return nil
	}

	err = a.agent.ServiceRegister(&consul.AgentServiceRegistration{
		ID:   "sked-" + h.Name,
		Name: "sked",
		Checks: consul.AgentServiceChecks{
			&consul.AgentServiceCheck{
				Interval: "30s",
				HTTP:     h.HealthCheck,
			},
		},
	})
	if err != nil {
		return err
	}

	a.registered = true
	return nil
}

func (a *ConsulApi) DelHost(cluster, name string) error {
	return a.del("hosts/" + cluster + "/" + name)
}



// ==> TASK DEFINITION operations

func (a *ConsulApi) ListTaskDefinitions() (taskDefs []*api.TaskDefinition, err error) {
	list, err := a.list("config/task_definitions")
	if err != nil {
		return taskDefs, err
	}

	for _, v := range list {
		c := &api.TaskDefinition{}
		backends.Decode(v.Value, c)
		taskDefs = append(taskDefs, c)
	}
	return taskDefs, nil
}

func (a *ConsulApi) GetTaskDefinition(name string, version uint) (*api.TaskDefinition, error) {
	t := &api.TaskDefinition{}
	id := fmt.Sprintf("%s%s/%d", "config/task_definitions", name, version)

	kv, err := a.get(id)
	if kv == nil || kv.Value == nil {
		return t, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, t)
	}

	return t, err
}

func (a *ConsulApi) PutTaskDefinition(t *api.TaskDefinition) error {
	id := fmt.Sprintf("%s%s/%d", "config/task_definitions", t.Name, t.Version)
	return a.put(id, backends.Encode(t))
}




// ==> TASK operations

func (a *ConsulApi) ListTasks(q *api.TaskQueryOpts) (ts []*api.Task, err error) {

	prefix := "state/tasks/"
	if q.ByCluster != "" && q.ByDeployment != "" {
		prefix += q.ByCluster + "-" + q.ByDeployment
	} else if q.ByCluster != "" {
		prefix += q.ByCluster + "-"
	} else {
		log.Warn("[consul-api] iterating all tasks")
	}

	list, err := a.list(prefix)
	if err != nil {
		return ts, err
	}

	log.WithField("prefix", prefix).WithField("count", len(list)).Debug("[consul-api] query tasks")

	for _, v := range list {
		t := &api.Task{}
		backends.Decode(v.Value, t)

		if q.Failing || q.Running {
			state, err := a.GetTaskState(t.ID())
			if err != nil {
				return ts, err
			}

			if q.Failing && state.Healthy() {
				continue
			} else if q.Running && !state.Healthy(){
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

func (a *ConsulApi) GetTask(id string) (*api.Task, error) {
	t := &api.Task{}

	kv, err := a.get("state/tasks/" + id)
	if err == nil {
		backends.Decode(kv.Value, t)
	}

	return t, err
}

func (a *ConsulApi) PutTask(t *api.Task) error {
	err := a.register(t)
	if err != nil {
		log.WithField("err", err).Warn("[consul-api] error registering task")
	}
	return a.put("state/tasks/"+t.ID(), backends.Encode(t))
}

func (a *ConsulApi) DelTask(t *api.Task) error {
	a.deRegister(t.ID())
	return a.del("state/tasks/" + t.ID())
}



// TASK Health Operations

func (a *ConsulApi) GetTaskState(taskId string) (api.TaskState, error) {
	s, _, err := a.health.Checks("task-"+taskId, nil)
	if err != nil {
		return false, err
	}

	stateStr, err := a.get("state/health/" + taskId)
	if err != nil && err != api.ErrNotFound {
		return api.FAILING, err
	}

	state := api.TaskState(stateStr)

	// authoritative over these states only
	if state == api.STOPPING || state == api.STARTING || state == api.EXITED {
		return state, nil
	}

	any := false
	for _, ch := range s {
		if ch.ServiceID == taskId {
			any = true
			if ch.Status != "passing" {
				return api.FAILING, nil
			}
		}
	}

	if any {
		return api.RUNNING, nil
	}

	return state, nil
}

func (a *ConsulApi) PutTaskState(taskId string, s api.TaskState) (bool, error) {
	return a.put("state/health/" + taskId, s)
}


func (a *ConsulApi) Debug() {
	list, _, _ := a.kv.List("", nil)
	for _, kv := range list {
		fmt.Printf("-> %s\n", kv.Key)
	}
}

func (a *ConsulApi) register(t *api.Task) error {

	checks := consul.AgentServiceChecks{}

	for _, cont := range t.TaskDefinition.Containers {
		for _, check := range cont.Checks {
			consulCheck := &consul.AgentServiceCheck{
				Interval: fmt.Sprintf("%vs", check.Interval.Seconds()),
				Script:   check.Script,
				Timeout:  fmt.Sprintf("%vs", check.Timeout.Seconds()),
				TCP:      check.TCPWithPort(t, cont),
				HTTP:     check.HTTPWithPort(t, cont),
			}

			if check.Docker != "" {
				consulCheck.DockerContainerID = t.Id()
				consulCheck.Script = check.Docker
			}

			fmt.Printf("checks: %+v\n", consulCheck)
			checks = append(checks, consulCheck)
		}
	}


	log.WithField("id", t.Id()).WithField("name", t.Name()).WithField("checks", len(checks)).WithField("host", t.Host).Debug("registering task")

	return a.agent.ServiceRegister(&consul.AgentServiceRegistration{
		ID:      "task-" + t.ID(),
		Name:    t.Name(),
		Tags:    append(t.TaskDefinition.Tags, t.Cluster, t.Deployment),
		Address: t.Host,
		Checks:  checks,
	})
}

func (a *ConsulApi) deRegister(taskId string) error {
	return a.agent.ServiceDeregister("task-" + taskId)
}
