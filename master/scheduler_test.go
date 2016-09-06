package master

import (
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"fmt"
	"testing"
	"time"
)

func testScheduler(t *testing.T, clusterName, serviceName, file string, hosts int) *api.MockApi {
	a := api.NewMockApi()
	s := NewDefaultScheduler(a)

	for i := 0; i < hosts; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	actions.ApplyConfig(file, a)

	cluster, err := a.GetCluster(clusterName)
	tools.Ok(t, err)

	service, err := a.GetService(serviceName)
	tools.Ok(t, err)

	tools.Ok(t, s.Schedule("", cluster, service))
	return a
}

func assertCountServices(t *testing.T, a *api.MockApi, service string, count int) {
	l, _ := a.ListTasks(&api.TaskQueryOpts{
		ByService: service,
		Scheduled: true,
	})

	tools.Assert(t, len(l) == count, fmt.Sprintf("running services %d, expected: %d", len(l), count))
}

func assertCountScheduledOn(t *testing.T, a *api.MockApi, host string, count int) {
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
	testScheduler(t, "default", "helloworld", "../examples/complex.yml", 5)
}

// test concurrency
func TestScheduler_ConcurrentScheduling(t *testing.T) {
	a := api.NewMockApi()
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
	cl.Services = []string{"test1", "test2", "test3"}
	a.PutCluster(cl)

	ser1 := api.SampleService()
	ser1.Name = "test1"
	a.PutService(ser1)

	ser2 := api.SampleService()
	ser2.Name = "test2"
	a.PutService(ser2)

	ser3 := api.SampleService()
	ser3.Name = "test3"
	a.PutService(ser3)

	go tools.Ok(t, s.Schedule("binpack", cl, ser1))
	go tools.Ok(t, s.Schedule("binpack", cl, ser2))
	go tools.Ok(t, s.Schedule("binpack", cl, ser3))

	time.Sleep(1 * time.Second)

	portsHost := make(map[string][]uint)
	ts, _ := a.ListTasks(&api.TaskQueryOpts{})
	for _, task := range ts {
		for _, p := range task.AllPorts() {
			tools.Assert(t, !inArray(p, portsHost[task.Host]), fmt.Sprintf("twice alloc: %v %v", portsHost[task.Host], task.AllPorts()))
		}

		portsHost[task.Host] = append(portsHost[task.Host], task.AllPorts()...)
	}
}

// test scaling
func TestScheduler_Scaling(t *testing.T) {
	a := api.NewMockApi()
	s := NewDefaultScheduler(a)

	for i := 0; i < 10; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	a.PutTaskDefinition(api.SampleTaskDefinition())

	cl := api.SampleCluster()
	cl.Services = []string{"test"}
	a.PutCluster(cl)

	ser := api.SampleService()
	ser.Desired = 2
	ser.Max = 5
	ser.Min = 1
	ser.Name = "test"
	a.PutService(ser)

	fmt.Println("-> schedule for 2")
	tools.Ok(t, s.Schedule("spread", cl, ser))
	assertCountServices(t, a, "test", 2)

	ser.Desired = 1
	a.PutService(ser)

	fmt.Println("-> schedule for 1")
	tools.Ok(t, s.Schedule("spread", cl, ser))
	assertCountServices(t, a, "test", 1)

	ser.Desired = 5
	a.PutService(ser)

	fmt.Println("-> schedule for 5")
	tools.Ok(t, s.Schedule("spread", cl, ser))
	assertCountServices(t, a, "test", 5)
}

// test draining host
func TestScheduler_DrainingHost(t *testing.T) {
	a := api.NewMockApi()
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
	cl.Services = []string{"test"}
	a.PutCluster(cl)

	ser := api.SampleService()
	ser.Desired = 2
	ser.Max = 5
	ser.Min = 1
	ser.Name = "test"
	a.PutService(ser)

	tools.Ok(t, s.Schedule("spread", cl, ser))

	assertCountScheduledOn(t, a, "local-drain", 0)
}

func TestScheduler_GC(t *testing.T) {
	a := api.NewMockApi()
	s := NewDefaultScheduler(a)

	h := api.SampleHost()
	h.Name = "local"
	a.PutHost(h)

	a.PutTaskDefinition(api.SampleTaskDefinition())

	cl := api.SampleCluster()
	cl.Services = []string{"test"}
	a.PutCluster(cl)

	ser := api.SampleService()
	ser.Desired = 2
	ser.Max = 5
	ser.Min = 1
	ser.Name = "test"
	a.PutService(ser)

	tools.Ok(t, s.Schedule("spread", cl, ser))
	assertCountServices(t, a, "test", 2)

	cl.Services = []string{}
	a.PutCluster(cl)

	m := NewMaster(a, &Config{Cluster: cl.Name})

	m.runGarbageCollect()
	assertCountServices(t, a, "test2", 0)
}
