package etcd

import (
	"golang.org/x/net/context"
	"github.com/coreos/etcd/client"
	"github.com/coldog/sked/backends"
	"github.com/coldog/sked/api"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"sync"
	"os"
)

func NewEtcdApi(prefix string, conf client.Config) *EtcdApi {
	c, err := client.New(conf)
	if err != nil {
		panic(err)
	}

	return &EtcdApi{
		prefix: prefix,
		kv: client.NewKeysAPI(c),
		eventLock: &sync.RWMutex{},
		listeners: map[string]*listener{},
	}
}

type EtcdApi struct {
	kv client.KeysAPI
	prefix string
	listeners map[string]*listener
	eventLock *sync.RWMutex
	hostname  string
}

func (a *EtcdApi) put(key string, val []byte) error {
	_, err := a.kv.Set(context.Background(), a.prefix+key, string(val), nil)
	return err
}

func (a *EtcdApi) del(key string) error {
	_, err := a.kv.Delete(context.Background(), a.prefix+key, nil)
	return err
}

func (a *EtcdApi) get(key string) ([]byte, error) {
	res, err := a.kv.Get(context.Background(), a.prefix+key, &client.GetOptions{Recursive: false})
	if err != nil {
		return nil, err
	}

	if res == nil || res.Node.Value == "" {
		return nil, api.ErrNotFound
	}

	return []byte(res.Node.Value), nil
}

func (a *EtcdApi) list(key string, each func(val []byte) error) error {
	res, err := a.kv.Get(context.Background(), a.prefix+key, &client.GetOptions{Recursive: true})
	if err != nil {
		return err
	}

	for _, v := range res.Node.Nodes {
		err := each([]byte(v.Value))
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *EtcdApi) HostName() (string, error) {
	if a.hostname != "" {
		return a.hostname, nil
	}

	h, err := os.Hostname()
	if err != nil {
		return "", err
	}

	a.hostname = h
	return h, nil
}



// ==> CLUSTER operations

func (a *EtcdApi) ListClusters() (clusters []*api.Cluster, err error) {
	err = a.list("config/clusters", func(v []byte) error {
		c := &api.Cluster{}
		backends.Decode(v, c)
		clusters = append(clusters, c)
		return nil
	})
	return
}

func (a *EtcdApi) GetCluster(id string) (*api.Cluster, error) {
	c := &api.Cluster{}
	v, err := a.get("config/clusters/" + id)
	if err == nil {
		backends.Decode(v, c)
	}

	return c, err
}

func (a *EtcdApi) PutCluster(c *api.Cluster) error {
	return a.put("config/clusters/" + c.Name, backends.Encode(c))
}

func (a *EtcdApi) DelCluster(id string) error {
	return a.del("config/clusters/" + id)
}



// ==> DEPLOYMENT operations

func (a *EtcdApi) ListDeployments() (deps []*api.Deployment, err error) {
	err = a.list("config/deployments", func(v []byte) error {
		c := &api.Deployment{}
		backends.Decode(v, c)
		deps = append(deps, c)
		return nil
	})
	return deps, err
}

func (a *EtcdApi) GetDeployment(id string) (*api.Deployment, error) {
	c := &api.Deployment{}

	v, err := a.get("config/deployments/" + id)
	if err == nil {
		backends.Decode(v, c)
	}

	return c, err
}

func (a *EtcdApi) PutDeployment(c *api.Deployment) error {
	return a.put("config/deployments/" + c.Name, backends.Encode(c))
}

func (a *EtcdApi) DelDeployment(id string) error {
	return a.del("config/deployments/" + id)
}



// ==> SERVICE operations

func (a *EtcdApi) ListServices() (services []*api.Service, err error) {
	err = a.list("config/services", func(v []byte) error {
		c := &api.Service{}
		backends.Decode(v, c)
		services = append(services, c)
		return nil
	})
	return
}

func (a *EtcdApi) GetService(id string) (*api.Service, error) {
	c := &api.Service{}

	v, err := a.get("config/services/" + id)
	if err == nil {
		backends.Decode(v, c)
	}

	return c, err
}

func (a *EtcdApi) PutService(s *api.Service) error {
	return a.put("config/services/"+s.Name, backends.Encode(s))
}

func (a *EtcdApi) DelService(id string) error {
	return a.del("config/services/" + id)
}


// ==> HOST operations

func (a *EtcdApi) ListHosts(opts *api.HostQueryOpts) (hosts []*api.Host, err error) {
	err = a.list("hosts/" + opts.ByCluster, func(v []byte) error {
		h := &api.Host{}
		backends.Decode(v, h)
		hosts = append(hosts, h)
		return nil
	})
	return
}

func (a *EtcdApi) GetHost(cluster, name string) (*api.Host, error) {
	h := &api.Host{}

	v, err := a.get("hosts/" + cluster + "/" + name)
	if err == nil {
		backends.Decode(v, h)
	}

	return h, err
}

func (a *EtcdApi) PutHost(h *api.Host) error {
	return a.put("hosts/"+h.Cluster+"/"+h.Name, backends.Encode(h))
}

func (a *EtcdApi) DelHost(cluster, name string) error {
	return a.del("hosts/" + cluster + "/" + name)
}



// ==> TASK DEFINITION operations

func (a *EtcdApi) ListTaskDefinitions() (taskDefs []*api.TaskDefinition, err error) {
	err = a.list("config/task_definitions/", func(v []byte) error {
		c := &api.TaskDefinition{}
		backends.Decode(v, c)
		taskDefs = append(taskDefs, c)
		return nil
	})
	return
}

func (a *EtcdApi) GetTaskDefinition(name string, version uint) (*api.TaskDefinition, error) {
	t := &api.TaskDefinition{}
	id := fmt.Sprintf("%s%s/%d", "config/task_definitions/", name, version)

	v, err := a.get(id)
	if err == nil {
		backends.Decode(v, t)
	}

	return t, err
}

func (a *EtcdApi) PutTaskDefinition(t *api.TaskDefinition) error {
	id := fmt.Sprintf("%s%s/%d", "config/task_definitions/", t.Name, t.Version)
	return a.put(id, backends.Encode(t))
}



// ==> TASK operations

func (a *EtcdApi) ListTasks(q *api.TaskQueryOpts) (ts []*api.Task, err error) {

	prefix := "state/tasks/"
	if q.ByCluster != "" && q.ByDeployment != "" {
		prefix += q.ByCluster + "-" + q.ByDeployment
	} else if q.ByCluster != "" {
		prefix += q.ByCluster + "-"
	} else {
		log.Warn("[consul-api] iterating all tasks")
	}


	err = a.list(prefix, func(v []byte) error {
		t := &api.Task{}
		backends.Decode(v, t)

		if q.Failing || q.Running {
			state, err := a.GetTaskState(t)
			if err != nil {
				return err
			}

			if q.Failing && state.Healthy() {
				return nil
			} else if q.Running && !state.Healthy(){
				return nil
			}
		}

		if q.Scheduled && !t.Scheduled {
			return nil
		}

		if q.Rejected && !t.Rejected {
			return nil
		}

		if q.ByHost != "" && t.Host != q.ByHost {
			return nil
		}

		if q.ByCluster != "" && q.ByCluster != t.Cluster {
			return nil
		}

		if q.ByDeployment != "" && q.ByDeployment != t.Deployment {
			return nil
		}

		ts = append(ts, t)
		return nil
	})
	return
}

func (a *EtcdApi) GetTask(id string) (*api.Task, error) {
	t := &api.Task{}

	v, err := a.get("state/tasks/" + id)
	if err == nil {
		backends.Decode(v, t)
	}

	return t, err
}

func (a *EtcdApi) PutTask(t *api.Task) error {
	return a.put("state/tasks/"+t.ID(), backends.Encode(t))
}

func (a *EtcdApi) DelTask(t *api.Task) error {
	return a.del("state/tasks/" + t.ID())
}



// TASK Health Operations

func (a *EtcdApi) GetTaskState(t *api.Task) (api.TaskState, error) {
	raw, err := a.get("state/health/" + t.ID())
	return api.TaskState(string(raw)), err
}

func (a *EtcdApi) PutTaskState(taskId string, s api.TaskState) error {
	return a.put("state/health/" + taskId, []byte(s))
}