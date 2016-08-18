package api

import (
	"github.com/coldog/scheduler/tools"
	"testing"
)

// HostName() (string, error)
func TestConsulApi_HostName(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		_, err := api.HostName()
		tools.Ok(t, err)
	})
}

//Lock(key string) (Lockable, error)
func TestConsulApi_Lock(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		lock, err := api.Lock("test")
		tools.Ok(t, err)

		_, err = lock.Lock(nil)
		tools.Ok(t, err)

		err = lock.Unlock()
		tools.Ok(t, err)
	})
}

//Register(t *Task) error
func TestConsulApi_Register(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.Register(sampleTask())
		tools.Ok(t, err)

		err = api.DeRegister(sampleTask().Id())
		tools.Ok(t, err)
	})
}

//ListClusters() ([]*Cluster, error)
//GetCluster(id string) (*Cluster, error)
//PutCluster(cluster *Cluster) error
//DelCluster(id string) error
func TestConsulApi_Cluster(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutCluster(sampleCluster())
		tools.Ok(t, err)

		c, err := api.GetCluster(sampleCluster().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, sampleCluster().Name)

		cs, err := api.ListClusters()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no clusters to list")

		errr := api.DelCluster(sampleCluster().Name)
		tools.Ok(t, errr)
	})
}

//ListServices() ([]*Service, error)
//GetService(id string) (*Service, error)
//PutService(s *Service) (*Service, error)
//DelService(id string) (*Service, error)
func TestConsulApi_Services(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutService(sampleService())
		tools.Ok(t, err)

		c, err := api.GetService(sampleService().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, sampleService().Name)

		cs, err := api.ListServices()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := api.DelService(sampleService().Name)
		tools.Ok(t, errr)
	})
}

//ListTaskDefinitions() ([]*TaskDefinition, error)
//GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
//PutTaskDefinition(t *TaskDefinition) error
func TestConsulApi_TaskDefinitions(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutTaskDefinition(sampleTaskDefinition())
		tools.Ok(t, err)

		c, err := api.GetTaskDefinition(sampleTaskDefinition().Name, sampleTaskDefinition().Version)
		tools.Ok(t, err)
		tools.Assert(t, c != nil, "task definition is nil")

		tools.Equals(t, c.Name, sampleTaskDefinition().Name)
		tools.Equals(t, c.Version, sampleTaskDefinition().Version)

		cs, err := api.ListTaskDefinitions()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no task definitions to list")

		_, err = api.GetTaskDefinition("non-existent", 100)
		tools.Assert(t, err == ErrNotFound, "found a non existent task definition")
	})
}

//ListHosts() ([]*Host, error)
//GetHost(id string) (*Host, error)
//PutHost(h *Host) (error)
//DelHost(id string) (error)
func TestConsulApi_Hosts(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutHost(sampleHost())
		tools.Ok(t, err)

		c, err := api.GetHost(sampleHost().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, sampleHost().Name)

		cs, err := api.ListHosts()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := api.DelHost(sampleHost().Name)
		tools.Ok(t, errr)
	})
}

//ListTasks(q *TaskQueryOpts) ([]*Task, error)
//GetTask(id string) ([]*Task, error)
//ScheduleTask(task *Task) error
//DeScheduleTask(task *Task) error
