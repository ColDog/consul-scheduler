package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"

	"errors"
	"fmt"
	"time"
)

var (
	ConfigPrefix     string = "config/"
	ClustersPrefix   string = ConfigPrefix + "clusters/"
	ServicesPrefix   string = ConfigPrefix + "services/"
	HostsPrefix      string = ConfigPrefix + "hosts/"
	TasksPrefix      string = ConfigPrefix + "tasks/"
	SchedulersPrefix string = "schedulers/"
	StatePrefix      string = "state/"
)

var NotFoundErr error = errors.New("NotFound")

type Config struct {
	ConsulApiAddress  string
	ConsulApiDc       string
	ConsulApiToken    string
	ConsulApiWaitTime time.Duration
}

func NewSchedulerApiWithConfig(conf *Config) *SchedulerApi {
	apiConfig := api.DefaultConfig()

	if conf.ConsulApiDc != "" {
		apiConfig.Datacenter = conf.ConsulApiDc
	}

	if conf.ConsulApiAddress != "" {
		apiConfig.Address = conf.ConsulApiAddress
	}

	if conf.ConsulApiToken != "" {
		apiConfig.Token = conf.ConsulApiToken
	}

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
	}

	return a
}

func NewSchedulerApi() *SchedulerApi {
	return NewSchedulerApiWithConfig(&Config{})
}

type SchedulerApi struct {
	kv         *api.KV
	agent      *api.Agent
	catalog    *api.Catalog
	health     *api.Health
	client     *api.Client
	ConsulConf *api.Config
}

// Loops and checks various api calls to see if consul is initialized. Sometimes if the cluster of consul servers
// is just starting it will return 500 until a group and a leader is elected.
func (a *SchedulerApi) WaitForInitialize() {
	for {
		_, err := a.Host()
		if err == nil {
			return
		}

		_, err = a.ListClusters()
		if err == nil {
			return
		}

		log.WithField("error", err).Error("consul has not started...")
		time.Sleep(15 * time.Second)
	}
}

func (a *SchedulerApi) put(key string, value []byte, flags ...uint64) error {
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

func (a *SchedulerApi) del(key string) error {
	_, err := a.kv.Delete(key, nil)
	if err != nil {
		log.WithField("consul-api", "del").WithField("key", key).Error(err)
	}
	return err
}

func (a *SchedulerApi) get(key string) (*api.KVPair, error) {
	res, _, err := a.kv.Get(key, nil)
	if err != nil {
		log.WithField("consul-api", "get").WithField("key", key).Error(err)
	}
	return res, err
}

func (a *SchedulerApi) list(prefix string) (api.KVPairs, error) {
	res, _, err := a.kv.List(prefix, nil)
	if err != nil {
		log.WithField("consul-api", "put").WithField("key", prefix).Error(err)
	}
	return res, err
}

func (a *SchedulerApi) Lock(key string) (*api.Lock, error) {
	lock, err := a.client.LockKey(key)
	if err != nil {
		log.Error(err)
	}
	return lock, err
}

func (a *SchedulerApi) WaitOnKey(key string) error {
	lastIdx := uint64(0)
	for {
		keys, meta, err := a.kv.List(key, &api.QueryOptions{
			AllowStale: true,
			WaitTime:   3 * time.Minute,
			WaitIndex:  lastIdx,
		})

		if err != nil {
			log.Error(err)
			return err
		}

		if lastIdx != 0 {
			for _, kv := range keys {
				if kv.ModifyIndex > lastIdx {
					return nil
				}
			}
		} else {
			for _, kv := range keys {
				if kv.ModifyIndex > lastIdx {
					lastIdx = kv.ModifyIndex
				}

				if kv.CreateIndex > lastIdx {
					lastIdx = kv.CreateIndex
				}
			}

			if lastIdx == uint64(0) {
				lastIdx = meta.LastIndex
			}
		}
	}
}

func (a *SchedulerApi) Host() (string, error) {
	n, err := a.agent.NodeName()
	if err != nil {
		return "", err
	}

	return n, nil
}

func (a *SchedulerApi) RegisterAgent(id string) error {
	return a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID:   "consul-scheduler-" + id,
		Name: "consul-scheduler",
		Checks: api.AgentServiceChecks{
			&api.AgentServiceCheck{
				HTTP:     "http://127.0.0.1:8231/health",
				Interval: "15s",
			},
		},
	})
}

func (a *SchedulerApi) DeRegisterAgent(id string) error {
	return a.agent.ServiceDeregister("consul-scheduler-" + id)
}

func (a *SchedulerApi) Register(t Task) error {
	checks := api.AgentServiceChecks{}
	for _, check := range t.TaskDef.Checks {
		checks = append(checks, &api.AgentServiceCheck{
			Interval: check.Interval,
			Script:   check.Script,
			Timeout:  check.Timeout,
			TCP:      check.Tcp,
			HTTP:     check.Http,
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

func (a *SchedulerApi) DeRegister(taskId string) error {
	return a.agent.ServiceDeregister(taskId)
}

// ==> TASKS:
func (a *SchedulerApi) PutTask(t Task) error {
	// todo: wrap in transaction

	data := encode(t)
	err := a.put(StatePrefix+t.Host+"/"+t.Id(), data)
	err = a.put(StatePrefix+t.Id(), data)

	return err
}

func (a *SchedulerApi) IsTaskScheduled(taskId string) bool {
	res, err := a.get("state/" + taskId)
	return err == nil && res != nil && res.Flags == uint64(0)
}

// Deleting a task flags the task in consul as having a 1
func (a *SchedulerApi) DelTask(t Task) error {
	// todo: wrap in a transaction

	err := a.del(StatePrefix + t.Host + "/" + t.Id())
	err = a.put(StatePrefix+t.Id(), encode(t), uint64(1))
	return err
}

func (a *SchedulerApi) ListTasks(prefix string) (tasks []Task, err error) {
	list, err := a.list(StatePrefix + prefix)
	if err != nil {
		return tasks, err
	}

	for _, kv := range list {
		t := Task{}
		decode(kv.Value, &t)
		t.Stopped = kv.Flags == uint64(1)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (a *SchedulerApi) GetTask(taskId string) (t Task, err error) {
	res, err := a.get(StatePrefix + taskId)
	if err != nil {
		return t, err
	}

	if res == nil {
		return t, NotFoundErr
	}

	decode(res.Value, &t)
	t.Stopped = res.Flags == uint64(1)
	return t, nil
}

func (a *SchedulerApi) TaskPassing(task Task) (ok bool) {
	if task.Service == "" {
		return false
	}

	s, _, err := a.health.Checks(task.Name(), nil)
	if err != nil {
		panic(err)
	}

	for _, ch := range s {
		if ch.ServiceID == task.Id() {
			return ch.Status == "passing"
		}
	}

	return ok
}

// ==> SERVICES:
func (a *SchedulerApi) PutService(s Service) error {
	return a.put(ServicesPrefix+s.Name, encode(s))
}

func (a *SchedulerApi) DelService(name string) error {
	return a.del(ServicesPrefix + name)
}

func (a *SchedulerApi) GetService(name string) (s Service, err error) {
	res, err := a.get(ServicesPrefix + name)
	if err != nil {
		return s, err
	}

	if res == nil {
		return s, NotFoundErr
	}

	decode(res.Value, &s)
	return s, nil
}

// ==> HOSTS:
func (a *SchedulerApi) ListHosts() (hosts []Host, err error) {
	list, err := a.list(HostsPrefix)
	if err != nil {
		return hosts, err
	}

	for _, res := range list {
		host := Host{}
		decode(res.Value, &host)
		hosts = append(hosts, host)
	}

	return hosts, nil
}

func (a *SchedulerApi) GetHost(name string) (h Host, err error) {
	res, err := a.get(ServicesPrefix + name)
	if err != nil {
		return h, err
	}

	if res == nil {
		return h, NotFoundErr
	}

	decode(res.Value, &h)
	return h, nil
}

func (a *SchedulerApi) PutHost(host Host) error {
	return a.put(HostsPrefix+host.Name, encode(host))
}

func (a *SchedulerApi) DelHost(hostId string) error {
	return a.del(HostsPrefix + hostId)
}

// ==> TASK DEFINITIONS:
func (a *SchedulerApi) GetTaskDefinition(name string, ver uint) (s TaskDefinition, err error) {
	key := fmt.Sprintf("%s%s/%v", TasksPrefix, name, ver)

	res, err := a.get(key)
	if err != nil {
		return s, err
	}

	if res == nil {
		return s, NotFoundErr
	}

	decode(res.Value, &s)
	return s, nil
}

func (a *SchedulerApi) PutTaskDefinition(s TaskDefinition) error {
	key := fmt.Sprintf("%s%s/%v", TasksPrefix, s.Name, s.Version)
	return a.put(key, encode(s))
}

// ==> CLUSTERS:
func (a *SchedulerApi) ListClusters() (clusters []Cluster, err error) {
	list, err := a.list(ClustersPrefix)
	if err != nil {
		return clusters, err
	}

	for _, res := range list {
		cluster := Cluster{}
		decode(res.Value, &cluster)
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func (a *SchedulerApi) PutCluster(c Cluster) error {
	return a.put(ClustersPrefix+c.Name, encode(c))
}

func (a *SchedulerApi) GetCluster(name string) (c Cluster, err error) {
	kv, err := a.get(ClustersPrefix + name)
	if err == nil {
		decode(kv.Value, &c)
	}
	return c, err
}

// ==> TASK QUERIES:
func (a *SchedulerApi) RunningTasksOnHost(host string) (tasks map[string]RunningTask, err error) {
	tasks = make(map[string]RunningTask)

	s, err := a.agent.Services()
	if err != nil {
		return tasks, err
	}

	for _, ser := range s {
		t, err := a.GetTask(ser.ID)
		if err == nil {
			serv, _ := a.GetService(t.Service)

			tasks[ser.ID] = RunningTask{
				ServiceID: ser.ID,
				Task:      t,
				Exists:    err == nil,
				Service:   serv,
				Passing:   a.TaskPassing(t),
			}
		}
	}

	return tasks, nil
}

func (a *SchedulerApi) DesiredTasksByHost(host string) (tasks []Task, err error) {
	list, _, err := a.kv.List(StatePrefix+host+"/", nil)
	if err != nil {
		return tasks, err
	}

	for _, res := range list {
		t := Task{}
		decode(res.Value, &t)
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (a *SchedulerApi) TaskCount(prefix string) (int, error) {
	list, _, err := a.kv.List(StatePrefix+prefix, nil)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, kv := range list {
		if kv.Flags == uint64(0) {
			count++
		}
	}

	return count, nil
}

func (a *SchedulerApi) HealthyTaskCount(taskName string) (int, error) {
	count := 0

	e, _, err := a.health.Service(taskName, "", false, nil)
	if err != nil {
		return 0, err
	}

	for _, entry := range e {
		passing := true

		for _, check := range entry.Checks {
			if check.Status != "passing" {
				passing = false
				break
			}
		}

		if passing {
			count++
		}
	}

	return count, err
}
