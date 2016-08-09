package api

import (
	"github.com/hashicorp/consul/api"
	log "github.com/Sirupsen/logrus"

	"fmt"
	"time"
	"errors"
)

var NotFoundErr error = errors.New("NotFound")

type Config struct {
	ConsulApiAddress 	string
	ConsulApiDc 		string
	ConsulApiToken 		string
	ConsulApiWaitTime 	time.Duration
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

	apiConfig.WaitTime = conf.ConsulApiWaitTime

	client, err := api.NewClient(apiConfig)
	if err != nil {
		log.Fatal(err)
	}

	a := &SchedulerApi{
		kv: client.KV(),
		agent: client.Agent(),
		catalog: client.Catalog(),
		health: client.Health(),
		client: client,
	}

	return a
}

func NewSchedulerApi() *SchedulerApi {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}

	a := &SchedulerApi{
		kv: client.KV(),
		agent: client.Agent(),
		catalog: client.Catalog(),
		health: client.Health(),
		client: client,
	}

	return a
}

type SchedulerApi struct {
	kv 		*api.KV
	agent 		*api.Agent
	catalog 	*api.Catalog
	health 		*api.Health
	client 		*api.Client
}

func (a *SchedulerApi) LockScheduler() *api.Lock {
	lock, err := a.client.LockKey("scheduler")
	if err != nil {
		panic(err)
	}

	_, err = lock.Lock(nil)
	if err != nil {
		panic(err)
	}

	return lock
}

func (a *SchedulerApi) put(key string, value []byte, flags ...uint64) error {
	flag := uint64(0)
	if len(flags) >= 1 {
		flag = flags[0]
	}

	_, err := a.kv.Put(&api.KVPair{Key: key, Value: value, Flags: flag}, nil)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (a *SchedulerApi) del(key string) error {
	_, err := a.kv.Delete(key, nil)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (a *SchedulerApi) get(key string) (*api.KVPair, error) {
	res, _, err := a.kv.Get(key, nil)
	if err != nil {
		log.Error(err)
	}
	return res, err
}

func (a *SchedulerApi) list(prefix string) (api.KVPairs, error) {
	res, _, err := a.kv.List(prefix, nil)
	if err != nil {
		log.Error(err)
	}
	return res, err
}

func (a *SchedulerApi) WaitOnKey(key string) error {
	lastIdx := uint64(0)
	for {
		keys, _, err := a.kv.List(key, &api.QueryOptions{
			AllowStale: false,
			WaitTime: 10 * time.Minute,
			WaitIndex: lastIdx,
		})
		if err != nil {
			panic(err)
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
			}

			if lastIdx == 0 {
				lastIdx = uint64(10)
			}
		}
	}
}

func (a *SchedulerApi) Host() string {
	n, err := a.agent.NodeName()
	if err != nil {
		panic(err)
	}

	return n
}

func (a *SchedulerApi) ListClusters() (clusters []Cluster, err error) {
	list, err := a.list("cluster/")
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

func (a *SchedulerApi) RegisterAgent(id string) error {
	return a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID: "consul-scheduler-" + id,
		Name: "consul-scheduler",
		Checks: api.AgentServiceChecks{
			&api.AgentServiceCheck{
				HTTP: "http://127.0.0.1:8231",
				Interval: "60s",
			},
		},
	})
}

func (a *SchedulerApi) Register(t Task) error {
	servs, err := a.agent.Services()
	if err != nil {
		return err
	}

	for id, _ := range servs {
		if id == t.Id() {
			return nil
		}
	}

	checks := api.AgentServiceChecks{}

	for _, check := range t.TaskDef.Checks {

		if check.AddProvidedPort {
			if t.TaskDef.ProvidePort && check.Http != "" {
				check.Http = fmt.Sprintf("%s:%d", check.Http, t.Port)
			}

			if t.TaskDef.ProvidePort && check.Tcp != "" {
				check.Http = fmt.Sprintf("%s:%d", check.Tcp, t.Port)
			}
		}

		checks = append(checks, &api.AgentServiceCheck{
			Interval: check.Interval,
			Script: check.Script,
			Timeout: check.Timeout,
			TCP: check.Tcp,
			HTTP: check.Http,
		})
	}

	log.WithField("id", t.Id()).WithField("name", t.Name()).WithField("host", t.Host).Debug("registering task")

	err = a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID: t.Id(),
		Name: t.Name(),
		Tags: append(t.TaskDef.Tags, t.Cluster.Name, t.Service),
		Port: int(t.Port),
		Address: t.Host,
		Checks: checks,
	})

	return err
}

func (a *SchedulerApi) DeRegister(taskId string) error {
	return a.agent.ServiceDeregister(taskId)
}

func (a *SchedulerApi) PutTask(t Task) error {
	// todo: wrap in transaction

	data := encode(t)
	err := a.put("state/" + t.Host + "/" + t.Id(), data)
	err = a.put("state/" + t.Id(), data)

	log.WithField("task", t.Id()).WithField("host", t.Host).WithField("port", t.Port).Info("scheduled task")
	return err
}

func (a *SchedulerApi) IsTaskScheduled(taskId string) bool {
	res, err := a.get("state/" + taskId)
	return err == nil && res != nil && res.Flags == uint64(0)
}

func (a *SchedulerApi) DelTask(t Task) error {
	// todo: wrap in a transaction

	err := a.del("state/" + t.Host + "/" + t.Id())
	err = a.put("state/" + t.Id(), encode(t), uint64(1))
	return err
}

func (a *SchedulerApi) ListTasks(prefix string) (tasks []Task, err error) {
	list, err := a.list("state/" + prefix)
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

func (a *SchedulerApi) PutService(s Service) error {
	return a.put("service/" + s.Name, encode(s))
}

func (a *SchedulerApi) DelService(name string) error {
	return a.del("service/" + name)
}

func (a *SchedulerApi) GetService(name string) (s Service, err error) {
	res, err := a.get("service/" + name)
	if err != nil {
		return s, err
	}

	if res == nil {
		return s, NotFoundErr
	}

	decode(res.Value, &s)
	return s, nil
}

func (a *SchedulerApi) ListHosts() (hosts []Host, err error) {
	list, err := a.list("host/")
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
	res, err := a.get("service/" + name)
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
	return a.put("host/" + host.Name, encode(host))
}

func (a *SchedulerApi) DelHost(hostId string) error {
	return a.del("host/" + hostId)
}

func (a *SchedulerApi) GetTaskDefinition(name string, ver uint) (s TaskDefinition, err error) {
	key := fmt.Sprintf("task/%s/%v", name, ver)

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
	key := fmt.Sprintf("task/%s/%v", s.Name, s.Version)
	return a.put(key, encode(s))
}

func (a *SchedulerApi) PutCluster(c Cluster) error {
	return a.put("cluster/" + c.Name, encode(c))
}

func (a *SchedulerApi) DebugKeys() {
	list, _, err := a.kv.Keys("", "", nil)
	if err != nil {
		panic(err)
	}

	for _, key := range list {
		fmt.Printf("> %s\n", key)
	}
}

func (a *SchedulerApi) GetTask(taskId string) (t Task, err error) {
	res, err := a.get("state/" + taskId)
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

func (a *SchedulerApi) RunningTasksOnHost(host string) (tasks map[string] RunningTask, err error) {
	tasks = make(map[string] RunningTask)

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
				Task: t,
				Exists: err == nil,
				Service: serv,
				Passing: a.TaskPassing(t),
			}
		}
	}

	return tasks, nil
}

func (a *SchedulerApi) DesiredTasksByHost(host string) (tasks []Task, err error) {
	list, _, err := a.kv.List("state/" + host + "/", nil)
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
	list, _, err := a.kv.List("state/" + prefix, nil)
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
			count ++
		}
	}

	return count, err
}
