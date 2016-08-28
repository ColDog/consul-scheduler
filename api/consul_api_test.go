package api

import (
	"github.com/coldog/sked/tools"
	"github.com/hashicorp/consul/api"

	"fmt"
	"testing"
	"time"
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
		lock, err := api.Lock("test", true)
		tools.Ok(t, err)

		_, err = lock.Lock()
		tools.Ok(t, err)

		err = lock.Unlock()
		tools.Ok(t, err)
	})
}

func TestConsulApi_LockNoWait(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		lock, err := a.Lock("test", true)
		tools.Ok(t, err)

		lc, err := lock.Lock()
		tools.Ok(t, err)
		tools.Assert(t, lc != nil, "lock is not held")
		tools.Assert(t, lock.IsHeld(), "lock is not marked as held")

		// if locked should return immediately
		lock2, err := a.Lock("test", false)

		tools.Ok(t, err)

		t1 := time.Now().Unix()
		c, err := lock2.Lock()
		t2 := time.Now().Unix()

		tools.Assert(t, c == nil, "lock is marked as held")
		tools.Assert(t, !lock2.IsHeld(), "lock is marked as held")

		fmt.Printf("waited: %v\n", float64(t2-t1)/1000.00)

		err = lock2.Unlock()
		tools.Assert(t, err == api.ErrLockNotHeld, "unlock did not error as expected")

		lock.Unlock()
		tools.Assert(t, !lock.IsHeld(), "lock unlocking did not mark lock as unheld")
	})
}

//Register(t *Task) error
func TestConsulApi_Register(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.Register(SampleTask())
		tools.Ok(t, err)

		err = api.DeRegister(SampleTask().Id())
		tools.Ok(t, err)
	})
}

//ListClusters() ([]*Cluster, error)
//GetCluster(id string) (*Cluster, error)
//PutCluster(cluster *Cluster) error
//DelCluster(id string) error
func TestConsulApi_Cluster(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutCluster(SampleCluster())
		tools.Ok(t, err)

		c, err := api.GetCluster(SampleCluster().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, SampleCluster().Name)

		cs, err := api.ListClusters()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no clusters to list")

		errr := api.DelCluster(SampleCluster().Name)
		tools.Ok(t, errr)
	})
}

//ListServices() ([]*Service, error)
//GetService(id string) (*Service, error)
//PutService(s *Service) (*Service, error)
//DelService(id string) (*Service, error)
func TestConsulApi_Services(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutService(SampleService())
		tools.Ok(t, err)

		c, err := api.GetService(SampleService().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, SampleService().Name)

		cs, err := api.ListServices()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := api.DelService(SampleService().Name)
		tools.Ok(t, errr)
	})
}

//ListTaskDefinitions() ([]*TaskDefinition, error)
//GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
//PutTaskDefinition(t *TaskDefinition) error
func TestConsulApi_TaskDefinitions(t *testing.T) {
	RunConsulApiTest(func(api *ConsulApi) {
		err := api.PutTaskDefinition(SampleTaskDefinition())
		tools.Ok(t, err)

		c, err := api.GetTaskDefinition(SampleTaskDefinition().Name, SampleTaskDefinition().Version)
		tools.Ok(t, err)
		tools.Assert(t, c != nil, "task definition is nil")

		tools.Equals(t, c.Name, SampleTaskDefinition().Name)
		tools.Equals(t, c.Version, SampleTaskDefinition().Version)

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
		err := api.PutHost(SampleHost())
		tools.Ok(t, err)

		c, err := api.GetHost(SampleHost().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, SampleHost().Name)

		cs, err := api.ListHosts()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := api.DelHost(SampleHost().Name)
		tools.Ok(t, errr)
	})
}

//ListTasks(q *TaskQueryOpts) ([]*Task, error)
//GetTask(id string) ([]*Task, error)
//ScheduleTask(task *Task) error
//DeScheduleTask(task *Task) error
