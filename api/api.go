package api

import (
	"github.com/hashicorp/consul/api"
	log "github.com/Sirupsen/logrus"
	"fmt"
	"time"
)

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

func (a *SchedulerApi) TriggerScheduler() {
	p := &api.KVPair{
		Key: "schedule",
		Value: []byte{byte(1)},
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) FinishedScheduling() {
	p := &api.KVPair{
		Key: "schedule",
		Value: []byte{byte(0)},
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) WaitForScheduler() {
	lastIdx := uint64(0)
	for {
		kv, meta, err := a.kv.Get("schedule", &api.QueryOptions{
			AllowStale: true,
			WaitTime: 10 * time.Minute,
			WaitIndex: lastIdx,
		})
		if err != nil {
			panic(err)
		}

		lastIdx = meta.LastIndex
		if kv != nil && len(kv.Value) > 0 && kv.Value[0] == byte(1) {
			return
		}
	}
}

func (a *SchedulerApi) WaitOnKey(key string) {
	lastIdx := uint64(0)
	for {
		keys, meta, err := a.kv.Keys("schedule", "", &api.QueryOptions{
			AllowStale: true,
			WaitTime: 10 * time.Minute,
			WaitIndex: lastIdx,
		})
		if err != nil {
			panic(err)
		}

		lastIdx = meta.LastIndex
		if len(keys) > 0 {
			return
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

func (a *SchedulerApi) ListClusters() (clusters []Cluster) {
	list, _, err := a.kv.List("cluster/", nil)
	if err != nil {
		panic(err)
	}

	for _, res := range list {
		cluster := Cluster{}
		decode(res.Value, &cluster)
		clusters = append(clusters, cluster)
	}

	return clusters
}

func (a *SchedulerApi) Register(t Task) {
	servs, err := a.agent.Services()
	if err != nil {
		panic(err)
	}

	for id, _ := range servs {
		if id == t.Id() {
			return
		}
	}

	checks := api.AgentServiceChecks{}

	for _, check := range t.TaskDef.Checks {

		// todo: provide a nicer way of specifying this
		if t.TaskDef.ProvidePort && check.Http != "" {
			check.Http = fmt.Sprintf("http://127.0.0.1:%d", t.Port)
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
	for _, check := range checks {
		fmt.Printf("checks: %s %s\n", check.HTTP, check.Interval)
	}

	err = a.agent.ServiceRegister(&api.AgentServiceRegistration{
		ID: t.Id(),
		Name: t.Name(),
		Tags: []string{t.Cluster.Name, t.Service},
		Port: int(t.Port),
		Address: t.Host,
		Checks: checks,
	})

	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) DeRegister(task Task) {
	err := a.agent.ServiceDeregister(task.Id())
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) PutTask(t Task) {
	p := &api.KVPair{
		Key: "state/" + t.Host + "/" + t.Id(),
		Value: encode(t),
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}

	p = &api.KVPair{
		Key: "state/" + t.Id(),
		Value: encode(t),
	}

	_, err = a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}

	log.WithField("task", t.Id()).WithField("host", t.Host).Info("scheduled task")
}

func (a *SchedulerApi) IsTaskScheduled(taskId string) bool {
	res, _, err := a.kv.Get("state/" + taskId, nil)
	if err != nil {
		panic(err)
	}

	return res != nil
}

func (a *SchedulerApi) DelTask(t Task) {
	_, err := a.kv.Delete("state/" + t.Host + "/" + t.Id(), nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) PutService(s Service) {
	p := &api.KVPair{
		Key: "service/" + s.Name,
		Value: encode(s),
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) DelService(name string) {
	_, err := a.kv.Delete("service/" + name, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) GetService(name string) (s Service, ok bool) {
	p, _, err := a.kv.Get("service/" + name, nil)
	if err != nil {
		panic(err)
	}

	if p == nil {
		return s, false
	}

	decode(p.Value, &s)
	return s, true
}

func (a *SchedulerApi) ListServiceNames() []string {
	list, _, err := a.kv.Keys("service/", "", nil)
	if err != nil {
		panic(err)
	}

	return list
}

func (a *SchedulerApi) ListHosts() (hosts []Host) {
	list, _, err := a.kv.List("host/", nil)
	if err != nil {
		panic(err)
	}

	for _, res := range list {
		host := Host{}
		decode(res.Value, &host)
		hosts = append(hosts, host)
	}

	return hosts
}

func (a *SchedulerApi) GetHost(name string) (h Host, ok bool) {
	p, _, err := a.kv.Get("host/" + name, nil)
	if err != nil {
		panic(err)
	}

	if p == nil {
		return h, false
	}

	decode(p.Value, &h)
	return h, true
}

func (a *SchedulerApi) PutHost(host Host) {
	p := &api.KVPair{
		Key: "host/" + host.Name,
		Value: encode(host),
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) GetTaskDefinition(name string, ver uint) (s TaskDefinition, ok bool) {
	p, _, err := a.kv.Get(fmt.Sprintf("task/%s/%v", name, ver), nil)
	if err != nil {
		panic(err)
	}

	if p == nil {
		return s, false
	}

	decode(p.Value, &s)
	return s, true
}

func (a *SchedulerApi) PutTaskDefinition(s TaskDefinition) {
	p := &api.KVPair{
		Key: fmt.Sprintf("task/%s/%v", s.Name, s.Version),
		Value: encode(s),
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
}

func (a *SchedulerApi) ListTaskDefinitionNames() []string {
	list, _, err := a.kv.Keys("task/", "", nil)
	if err != nil {
		panic(err)
	}

	return list
}

func (a *SchedulerApi) PutCluster(c Cluster) {
	p := &api.KVPair{
		Key: "cluster/" + c.Name,
		Value: encode(c),
	}

	_, err := a.kv.Put(p, nil)
	if err != nil {
		panic(err)
	}
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

func (a *SchedulerApi) DebugState(host string) {
	list, _, err := a.kv.List("state/" + host, nil)
	if err != nil {
		panic(err)
	}

	for _, res := range list {
		fmt.Printf("state: %s\n", res.Key)
	}

}

func (a *SchedulerApi) GetTask(taskId string) (t Task, ok bool) {
	res, _, err := a.kv.Get("state/" + taskId, nil)
	if err != nil {
		panic(err)
	}

	if res == nil {
		return t, false
	}

	if res.Value != nil {
		ok = true
		decode(res.Value, &t)
	}

	return t, ok
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

func (a *SchedulerApi) RunningTasksOnHost(host string) (tasks map[string] RunningTask) {
	tasks = make(map[string] RunningTask)

	s, err := a.agent.Services()
	if err != nil {
		panic(err)
	}

	for _, ser := range s {
		if ser.ID != "consul" {

			t, ok := a.GetTask(ser.ID)

			var serv Service
			if ok {
				serv, _ = a.GetService(t.Service)
			}

			tasks[ser.ID] = RunningTask{
				ServiceID: ser.ID,
				Task: t,
				Exists: ok,
				Service: serv,
				Passing: a.TaskPassing(t),
			}
		}
	}

	return tasks
}

func (a *SchedulerApi) DesiredTasksByHost(host string) (tasks []Task) {
	list, _, err := a.kv.List("state/" + host + "/", nil)
	if err != nil {
		panic(err)
	}

	for _, res := range list {
		t := Task{}
		decode(res.Value, &t)
		tasks = append(tasks, t)
	}
	return tasks
}

func (a *SchedulerApi) HealthyTaskCount(taskName string) int {
	count := 0

	e, _, err := a.health.Service(taskName, "", false, nil)
	if err != nil {
		panic(err)
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

	return count
}
