package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"

	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
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
	registered bool
}

func NewConsulApi(conf *StorageConfig, apiConf *api.Config) *ConsulApi {
	client, err := api.NewClient(apiConf)
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
		conf:      DefaultStorageConfig(),
	}

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

func (a *ConsulApi) Lock(key string, block bool) (Lockable, error) {
	l, err := a.client.LockOpts(&api.LockOptions{
		Key:         key,
		LockTryOnce: !block,
	})

	lock := &ConsulLockWrapper{lock: l}
	return lock, err
}

// ==> REGISTER & DEREGISTER

func (a *ConsulApi) Register(t *Task) error {

	checks := api.AgentServiceChecks{}
	sp := fmt.Sprintf("%d", t.Port)

	for _, cont := range t.TaskDefinition.Containers {
		for _, check := range cont.Checks {
			consulCheck := &api.AgentServiceCheck{
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

			checks = append(checks, consulCheck)
		}
	}

	log.WithField("id", t.Id()).WithField("name", t.Name()).WithField("host", t.Host).Debug("registering task")

	return a.agent.ServiceRegister(&api.AgentServiceRegistration{
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
	err := a.put(a.conf.HostsPrefix+h.Name, encode(h))
	if err != nil {
		return err
	}

	if a.registered {
		return nil
	}

	err = a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID:   "sked-" + h.Name,
		Name: "sked",
		Checks: api.AgentServiceChecks{
			&api.AgentServiceCheck{
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
func (a *ConsulApi) ListTasks(q *TaskQueryOpts) (ts []*Task, err error) {

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
		t := &Task{}
		decode(v.Value, t)

		if q.Failing || q.Running {
			health, err := t.Healthy()
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

func (a *ConsulApi) GetTask(id string) (*Task, error) {
	t := &Task{}

	kv, err := a.get(a.conf.TasksPrefix + id)
	if err == nil {
		decode(kv.Value, t)
	}

	t.api = a
	return t, err
}

func (a *ConsulApi) PutTask(t *Task) error {
	body := encode(t)

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

func (a *ConsulApi) DelTask(t *Task) error {
	err := a.del(a.conf.TasksPrefix + t.Id())
	if err != nil {
		return err
	}

	return a.del(a.conf.TasksByHostPrefix + t.Host + "/" + t.Id())
}

// used to find out if a task is passing.
func (a *ConsulApi) TaskHealthy(t *Task) (bool, error) {
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
