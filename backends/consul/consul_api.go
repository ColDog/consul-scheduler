package consul

import (
	"github.com/coldog/sked/api"

	consul "github.com/hashicorp/consul/api"
	log "github.com/Sirupsen/logrus"

	"fmt"
	"strings"
	"sync"
	"time"
	"github.com/coldog/sked/backends"
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
	conf       *api.StorageConfig
	listeners  map[string]*listener
	eventLock  *sync.RWMutex
	registered bool
}

func NewConsulApi(conf *api.StorageConfig, apiConf *consul.Config) *ConsulApi {
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
		conf:      api.DefaultStorageConfig(),
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
		conf:       api.DefaultStorageConfig(),
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

	go a.monitor(a.conf.TaskDefinitionsPrefix, "config")
	go a.monitor(a.conf.ServicesPrefix, "config")
	go a.monitor(a.conf.ClustersPrefix, "config")
	go a.monitor(a.conf.StatePrefix, "state")
	go a.monitorHealth()
}

func (a *ConsulApi) HostName() (string, error) {
	return a.agent.NodeName()
}

func (a *ConsulApi) Conf() *api.StorageConfig {
	return a.conf
}

func (a *ConsulApi) put(key string, value []byte, flags ...uint64) error {
	flag := uint64(0)
	if len(flags) >= 1 {
		flag = flags[0]
	}

	_, err := a.kv.Put(&consul.KVPair{Key: key, Value: value, Flags: flag}, nil)
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

func (a *ConsulApi) get(key string) (*consul.KVPair, error) {
	res, _, err := a.kv.Get(key, nil)
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
	res, _, err := a.kv.List(prefix, nil)
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

func (a *ConsulApi) Register(t *api.Task) error {

	checks := consul.AgentServiceChecks{}
	sp := fmt.Sprintf("%d", t.Port)

	for _, cont := range t.TaskDefinition.Containers {
		for _, check := range cont.Checks {
			consulCheck := &consul.AgentServiceCheck{
				Interval: fmt.Sprintf("%vs", check.Interval.Seconds()),
				Script:   check.Script,
				Timeout:  fmt.Sprintf("%vs", check.Timeout.Seconds()),
				TCP:      strings.Replace(check.TCP, "$PROVIDED_PORT", sp, 1),
				HTTP:     strings.Replace(check.HTTP, "$PROVIDED_PORT", sp, 1),
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
		ID:      t.Id(),
		Name:    t.Name(),
		Tags:    append(t.TaskDefinition.Tags, t.Cluster, t.Service),
		Port:    int(t.Port),
		Address: t.Host,
		Checks:  checks,
	})
}

func (a *ConsulApi) DeRegister(taskId string) error {
	return a.agent.ServiceDeregister(taskId)
}

func (a *ConsulApi) AgentHealth(name string) (bool, error) {
	checks, _, err := a.health.Node(name, nil)
	if err != nil {
		return false, err
	}

	for _, c := range checks {
		if c.CheckID == "serfHealth" {
			return c.Status == "passing", nil
			break
		}
	}

	return false, nil
}

// ==> CLUSTER operations

func (a *ConsulApi) ListClusters() (clusters []*api.Cluster, err error) {
	list, err := a.list(a.conf.ClustersPrefix)
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

	kv, err := a.get(a.conf.ClustersPrefix + id)
	if kv == nil || kv.Value == nil {
		return c, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutService(s *api.Service) error {
	return a.put(a.conf.ServicesPrefix+s.Name, backends.Encode(s))
}

func (a *ConsulApi) DelService(id string) error {
	return a.del(a.conf.ServicesPrefix + id)
}

// ==> SERVICE operations

func (a *ConsulApi) ListServices() (services []*api.Service, err error) {
	list, err := a.list(a.conf.ServicesPrefix)
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

	kv, err := a.get(a.conf.ServicesPrefix + id)
	if kv == nil || kv.Value == nil {
		return c, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, c)
	}

	return c, err
}

func (a *ConsulApi) PutCluster(c *api.Cluster) error {
	return a.put(a.conf.ClustersPrefix+c.Name, backends.Encode(c))
}

func (a *ConsulApi) DelCluster(id string) error {
	return a.del(a.conf.ClustersPrefix + id)
}

// ==> HOST operations

func (a *ConsulApi) ListHosts() (hosts []*api.Host, err error) {
	list, err := a.list(a.conf.HostsPrefix)
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

func (a *ConsulApi) GetHost(name string) (*api.Host, error) {
	h := &api.Host{}

	kv, err := a.get(a.conf.HostsPrefix + name)
	if kv == nil || kv.Value == nil {
		return h, api.ErrNotFound
	}

	if err == nil {
		backends.Decode(kv.Value, h)
	}

	return h, err
}

func (a *ConsulApi) PutHost(h *api.Host) error {
	err := a.put(a.conf.HostsPrefix+h.Name, backends.Encode(h))
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

func (a *ConsulApi) DelHost(name string) error {
	return a.del(a.conf.HostsPrefix + name)
}

// ==> TASK DEFINITION operations

func (a *ConsulApi) ListTaskDefinitions() (taskDefs []*api.TaskDefinition, err error) {
	list, err := a.list(a.conf.TaskDefinitionsPrefix)
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
	id := fmt.Sprintf("%s%s/%d", a.conf.TaskDefinitionsPrefix, name, version)

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
	id := fmt.Sprintf("%s%s/%d", a.conf.TaskDefinitionsPrefix, t.Name, t.Version)
	return a.put(id, backends.Encode(t))
}

// ==> TASK operations
func (a *ConsulApi) ListTasks(q *api.TaskQueryOpts) (ts []*api.Task, err error) {

	prefix := ""
	if q.ByHost != "" {
		prefix += a.conf.TasksByHostPrefix + q.ByHost
	} else if q.ByCluster != "" && q.ByService != "" {
		prefix += a.conf.TasksPrefix + q.ByCluster + "-" + q.ByService
	} else if q.ByCluster != "" {
		prefix += a.conf.TasksPrefix + q.ByCluster + "-"
	} else {
		// log.Warn("[consul-api] iterating all tasks")
		prefix += a.conf.TasksPrefix
	}

	list, err := a.list(prefix)
	if err != nil {
		return ts, err
	}

	// log.WithField("prefix", prefix).WithField("count", len(list)).Debug("[consul-api] query tasks")

	for _, v := range list {
		t := &api.Task{}
		backends.Decode(v.Value, t)

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

func (a *ConsulApi) GetTask(id string) (*api.Task, error) {
	t := &api.Task{}

	kv, err := a.get(a.conf.TasksPrefix + id)
	if err == nil {
		backends.Decode(kv.Value, t)
	}

	return t, err
}

func (a *ConsulApi) PutTask(t *api.Task) error {
	body := backends.Encode(t)

	// todo: expensive to serialize and put in both
	// todo: should be in a transaction
	// todo: task definition is serialized as well, this is probably overkill

	err := a.put(a.conf.TasksPrefix+t.Id(), body)
	if err != nil {
		return err
	}

	err = a.put(a.conf.TasksByHostPrefix+t.Host+"/"+t.Id(), body)
	if err != nil {
		return err
	}
	return nil
}

func (a *ConsulApi) DelTask(t *api.Task) error {
	err := a.del(a.conf.TasksPrefix + t.Id())
	if err != nil {
		return err
	}

	return a.del(a.conf.TasksByHostPrefix + t.Host + "/" + t.Id())
}

// used to find out if a task is passing.
func (a *ConsulApi) TaskHealthy(t *api.Task) (bool, error) {
	s, _, err := a.health.Checks(t.Name(), nil)
	if err != nil {
		return false, err
	}

	any := false
	for _, ch := range s {
		if ch.ServiceID == t.Id() {
			any = true
			if ch.Status != "passing" {
				return false, nil
			}
		}
	}

	// if no checks are registered, tasks will always be healthy.
	return any, nil
}

func (a *ConsulApi) Debug() {
	list, _, _ := a.kv.List("", nil)
	for _, kv := range list {
		fmt.Printf("-> %s\n", kv.Key)
	}
}

func (a *ConsulApi) PutTaskHealth(taskId, status string) error {
	return nil
}
