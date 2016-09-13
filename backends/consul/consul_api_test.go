package consul

import (
	"github.com/coldog/sked/tools"
	consul "github.com/hashicorp/consul/api"

	"fmt"
	"testing"
	"time"
	"github.com/coldog/sked/api"
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
		tools.Assert(t, err == consul.ErrLockNotHeld, "unlock did not error as expected")

		lock.Unlock()
		tools.Assert(t, !lock.IsHeld(), "lock unlocking did not mark lock as unheld")
	})
}

//ListClusters() ([]*Cluster, error)
//GetCluster(id string) (*Cluster, error)
//PutCluster(cluster *Cluster) error
//DelCluster(id string) error
func TestConsulApi_Cluster(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		err := a.PutCluster(api.SampleCluster())
		tools.Ok(t, err)

		c, err := a.GetCluster(api.SampleCluster().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, api.SampleCluster().Name)

		cs, err := a.ListClusters()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no clusters to list")

		errr := a.DelCluster(api.SampleCluster().Name)
		tools.Ok(t, errr)
	})
}

//ListServices() ([]*Service, error)
//GetService(id string) (*Service, error)
//PutService(s *Service) (*Service, error)
//DelService(id string) (*Service, error)
func TestConsulApi_Services(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		err := a.PutService(api.SampleService())
		tools.Ok(t, err)

		c, err := a.GetService(api.SampleService().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, api.SampleService().Name)

		cs, err := a.ListServices()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := a.DelService(api.SampleService().Name)
		tools.Ok(t, errr)
	})
}

//ListTaskDefinitions() ([]*TaskDefinition, error)
//GetTaskDefinition(name string, version uint) (*TaskDefinition, error)
//PutTaskDefinition(t *TaskDefinition) error
func TestConsulApi_TaskDefinitions(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		err := a.PutTaskDefinition(api.SampleTaskDefinition())
		tools.Ok(t, err)

		c, err := a.GetTaskDefinition(api.SampleTaskDefinition().Name, api.SampleTaskDefinition().Version)
		tools.Ok(t, err)
		tools.Assert(t, c != nil, "task definition is nil")

		tools.Equals(t, c.Name, api.SampleTaskDefinition().Name)
		tools.Equals(t, c.Version, api.SampleTaskDefinition().Version)

		cs, err := a.ListTaskDefinitions()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no task definitions to list")

		_, err = a.GetTaskDefinition("non-existent", 100)
		tools.Assert(t, err == api.ErrNotFound, "found a non existent task definition")
	})
}

//ListHosts() ([]*Host, error)
//GetHost(id string) (*Host, error)
//PutHost(h *Host) (error)
//DelHost(id string) (error)
func TestConsulApi_Hosts(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		err := a.PutHost(api.SampleHost())
		tools.Ok(t, err)

		c, err := a.GetHost(api.SampleHost().Cluster, api.SampleHost().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, api.SampleHost().Name)

		cs, err := a.ListHosts(&api.HostQueryOpts{ByCluster: api.SampleHost().Cluster})
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := a.DelHost(api.SampleHost().Cluster, api.SampleHost().Name)
		tools.Ok(t, errr)
	})
}

//ListHosts() ([]*Host, error)
//GetHost(id string) (*Host, error)
//PutHost(h *Host) (error)
//DelHost(id string) (error)
func TestConsulApi_Deployments(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		err := a.PutDeployment(api.SampleDeployment())
		tools.Ok(t, err)

		c, err := a.GetDeployment(api.SampleDeployment().Name)
		tools.Ok(t, err)
		tools.Equals(t, c.Name, api.SampleDeployment().Name)

		cs, err := a.ListDeployments()
		tools.Ok(t, err)
		tools.Assert(t, len(cs) > 0, "no services to list")

		errr := a.DelDeployment(api.SampleDeployment().Name)
		tools.Ok(t, errr)
	})
}

//ListTasks(q *TaskQueryOpts) ([]*Task, error)
//GetTask(id string) ([]*Task, error)
//ScheduleTask(task *Task) error
//DeScheduleTask(task *Task) error
