package skedb

import (
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/scheduler"
	"github.com/coldog/sked/tools"

	"testing"
)

func TestSkedDB_Sync(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		err := actions.ApplyConfig("../examples/hello-world.yml", a)
		tools.Ok(t, err)

		c, err := a.GetCluster("default")
		tools.Ok(t, err)

		// register a host
		a.PutHost(&api.Host{
			Name:          "test",
			CpuUnits:      1000000000,
			Memory:        1000000000,
			PortSelection: []uint{1000, 1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010},
			ReservedPorts: []uint{2000},
		})

		scheduler.RunDefaultScheduler(c, a, nil)

		db := NewSkedDB(a)
		tools.Ok(t, db.Sync())

		v, err := db.Count("version", "2")
		tools.Ok(t, err)
		tools.Assert(t, v > 0, "count returned 0")
	})
}


