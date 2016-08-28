package skedb

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	log "github.com/Sirupsen/logrus"

	"testing"
	"time"
)

func init()  {
	log.SetLevel(log.DebugLevel)
}

func TestSkedDB_Sync(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		a.Start()
		db := NewSkedDB(a)

		time.Sleep(1 * time.Second)

		a.PutHost(&api.Host{
			Name:          "test",
			CpuUnits:      1000000000,
			Memory:        1000000000,
			PortSelection: []uint{1000, 1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009, 1010},
			ReservedPorts: []uint{2000},
		})

		time.Sleep(1 * time.Second)

		task := api.SampleTask()
		task.Host = "test"
		a.ScheduleTask(task)

		v, err := db.Count("tasks", "service", "helloworld")
		tools.Ok(t, err)
		tools.Assert(t, v > 0, "count returned 0")
	})
}


