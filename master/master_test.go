package master

import (
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"fmt"
	"testing"
	"time"
	"github.com/coldog/sked/backends/mock"
	"github.com/coldog/sked/scheduler"
)

func assertCountServices(t *testing.T, a *mock.MockApi, dep string, count int) {
	l, _ := a.ListTasks(&api.TaskQueryOpts{
		ByDeployment: dep,
		Scheduled: true,
	})

	tools.Assert(t, len(l) == count, fmt.Sprintf("running services %d, expected: %d", len(l), count))
}

func TestMaster_WillSchedule(t *testing.T) {
	a := mock.NewMockApi()

	m := NewMaster(a, &Config{Runners: 2})

	go func() {
		time.Sleep(1 * time.Second)
		m.dispatchAll()
		time.Sleep(1 * time.Second)
		m.Stop()
	}()

	for i := 0; i < 3; i++ {
		h := api.SampleHost()
		h.Name = fmt.Sprintf("local-%d", i)
		err := a.PutHost(h)
		tools.Ok(t, err)
	}

	actions.ApplyConfig("../examples/hello-world.yml", a)

	m.Run()
}

func TestMaster_GC(t *testing.T) {
	a := mock.NewMockApi()

	s := scheduler.NewDefaultScheduler(a)

	h := api.SampleHost()
	h.Name = "local"
	a.PutHost(h)

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
	a.PutDeployment(ser)

	tools.Ok(t, s.Schedule(ser))
	assertCountServices(t, a, "test", 2)

	cl.Deployments = []string{}
	a.PutCluster(cl)

	m := NewMaster(a, &Config{Cluster: cl.Name})

	m.runGarbageCollect()
	assertCountServices(t, a, "test2", 0)
}
