package scheduler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/actions"
	"github.com/coldog/scheduler/tools"

	"testing"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestScheduler_WithoutHosts(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		err := actions.ApplyConfig("../examples/hello-world.yml", a)
		tools.Ok(t, err)

		c, err := a.GetCluster("default")
		tools.Ok(t, err)

		RunDefaultScheduler(c, a, nil)

		// scheduling with no hosts added shouldn't result in any allocations
		tasks, err := a.ListTasks(&api.TaskQueryOpts{})
		tools.Ok(t, err)

		tools.Assert(t, len(tasks) == 0, "tasks scheduled")
	})
}

func TestScheduler_WithHosts(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		err := actions.ApplyConfig("../examples/hello-world.yml", a)
		tools.Ok(t, err)

		c, err := a.GetCluster("default")
		tools.Ok(t, err)

		// register a host
		a.PutHost(&api.Host{
			Name: "test",
			CpuUnits: 10000,
			Memory: 100000,
			PortSelection: []uint{1000, 1001, 1002, 1003, 1004, 1005, 1006, 1007},
			ReservedPorts: []uint{2000},
		})

		RunDefaultScheduler(c, a, nil)

		a.Debug()

		// scheduling with no hosts added shouldn't result in any allocations
		tasks, err := a.ListTasks(&api.TaskQueryOpts{})
		tools.Ok(t, err)

		tools.Assert(t, len(tasks) > 0, "no tasks scheduled")
	})
}