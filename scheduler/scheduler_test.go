package scheduler

import (
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	"github.com/coldog/sked/backends/mock"

	"fmt"
	"testing"
	"time"
)

func testScheduler(t *testing.T, clusterName, serviceName, file string, hosts int) *mock.MockApi {
	a := mock.NewMockApi()

	s := NewDefaultScheduler(a)

	for i := 0; i < hosts; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	tools.Ok(t, actions.ApplyConfig(file, a))

	cl, err := a.GetCluster(clusterName)
	tools.Ok(t, err)
	s.SetCluster(cl)

	dep, err := a.GetDeployment(serviceName)
	tools.Ok(t, err)

	tools.Ok(t, s.Schedule(dep))
	return a
}

func assertCountServices(t *testing.T, a *mock.MockApi, dep string, count int) {
	l, _ := a.ListTasks(&api.TaskQueryOpts{
		ByDeployment: dep,
		Scheduled: true,
	})

	tools.Assert(t, len(l) == count, fmt.Sprintf("running services %d, expected: %d", len(l), count))
}

func assertCountScheduledOn(t *testing.T, a *mock.MockApi, host string, count int) {
	l, _ := a.ListTasks(&api.TaskQueryOpts{
		ByHost:    host,
		Scheduled: true,
	})

	tools.Assert(t, len(l) == count, fmt.Sprintf("running tasks on host: %s - actual: %d, expected: %d", host, len(l), count))
}

func TestDefaultScheduler_Simple(t *testing.T) {
	testScheduler(t, "default", "helloworld", "../examples/hello-world.yml", 5)
}

func TestDefaultScheduler_Complex(t *testing.T) {
	testScheduler(t, "default", "helloworld", "../examples/complex.yml", 25)
}

// test concurrency
func TestScheduler_ConcurrentScheduling(t *testing.T) {
	a := mock.NewMockApi()

	s := NewDefaultScheduler(a)

	for i := 0; i < 10; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		h.CalculatedResources.Memory = 1000000000
		h.BaseResources.Memory = 1000000000
		h.ObservedResources.Memory = 1000000000
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	a.PutTaskDefinition(api.SampleTaskDefinition())

	cl := api.SampleCluster()
	cl.Deployments = []string{"test1", "test2", "test3"}
	a.PutCluster(cl)
	s.SetCluster(cl)

	ser1 := api.SampleDeployment()
	ser1.Name = "test1"
	ser1.Scheduler = "binpack"
	a.PutDeployment(ser1)

	ser2 := api.SampleDeployment()
	ser2.Name = "test2"
	ser2.Scheduler = "binpack"
	a.PutDeployment(ser2)

	ser3 := api.SampleDeployment()
	ser3.Name = "test3"
	ser3.Scheduler = "binpack"
	a.PutDeployment(ser3)

	go tools.Ok(t, s.Schedule(ser1))
	go tools.Ok(t, s.Schedule(ser1))
	go tools.Ok(t, s.Schedule(ser2))
	go tools.Ok(t, s.Schedule(ser2))
	go tools.Ok(t, s.Schedule(ser3))
	go tools.Ok(t, s.Schedule(ser3))

	time.Sleep(1 * time.Second)

	portsHost := make(map[string][]uint)
	ts, _ := a.ListTasks(&api.TaskQueryOpts{})
	for _, task := range ts {
		for _, p := range task.TaskDefinition.AllPorts() {
			tools.Assert(t, !inArray(p, portsHost[task.Host]), fmt.Sprintf("twice alloc: %v %v", portsHost[task.Host], task.TaskDefinition.AllPorts()))
		}

		portsHost[task.Host] = append(portsHost[task.Host], task.TaskDefinition.AllPorts()...)
	}
}

// test scaling
func TestScheduler_Scaling(t *testing.T) {
	a := mock.NewMockApi()

	s := NewDefaultScheduler(a)

	for i := 0; i < 10; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	a.PutTaskDefinition(api.SampleTaskDefinition())

	cl := api.SampleCluster()
	cl.Deployments = []string{"test"}
	a.PutCluster(cl)
	s.SetCluster(cl)

	ser := api.SampleDeployment()
	ser.Desired = 2
	ser.Max = 5
	ser.Min = 1
	ser.Name = "test"
	ser.Scheduler = "spread"
	a.PutDeployment(ser)

	fmt.Println("-> schedule for 2")
	tools.Ok(t, s.Schedule(ser))
	assertCountServices(t, a, "test", 2)

	ser.Desired = 1
	a.PutDeployment(ser)

	fmt.Println("-> schedule for 1")
	tools.Ok(t, s.Schedule(ser))
	assertCountServices(t, a, "test", 1)

	ser.Desired = 5
	a.PutDeployment(ser)

	fmt.Println("-> schedule for 5")
	tools.Ok(t, s.Schedule(ser))
	assertCountServices(t, a, "test", 5)
}

// test draining host
func TestScheduler_DrainingHost(t *testing.T) {
	a := mock.NewMockApi()

	s := NewDefaultScheduler(a)

	h := api.SampleHost()
	h.Name = "local"
	a.PutHost(h)

	hd := api.SampleHost()
	hd.Name = "local-drain"
	hd.Draining = true
	a.PutHost(hd)

	a.PutTaskDefinition(api.SampleTaskDefinition())

	cl := api.SampleCluster()
	cl.Deployments = []string{"test"}
	a.PutCluster(cl)
	s.SetCluster(cl)

	ser := api.SampleDeployment()
	ser.Desired = 2
	ser.Max = 5
	ser.Min = 1
	ser.Name = "test"
	ser.Scheduler = "spread"
	a.PutDeployment(ser)

	tools.Ok(t, s.Schedule(ser))

	assertCountScheduledOn(t, a, "local-drain", 0)
}
