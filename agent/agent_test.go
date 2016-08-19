package agent

import (
	log "github.com/Sirupsen/logrus"

	"github.com/coldog/scheduler/actions"
	"github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/scheduler"
	"github.com/coldog/scheduler/tools"

	"testing"
	"fmt"
	"time"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestAgent_Syncing(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		err := actions.ApplyConfig("../examples/hello-world.yml", a)
		tools.Ok(t, err)

		ag := NewAgent(a)
		ag.GetHostName()
		ag.RegisterAgent()
		ag.PublishState()

		fmt.Printf("%+v\n", ag.LastState)

		c, err := a.GetCluster("default")
		tools.Ok(t, err)
		scheduler.RunDefaultScheduler(c, a, nil)

		a.Debug()
		ag.sync()

		count := 0
		for {
			select {
			case <-time.After(3 * time.Second):
				t.Fatal("took too long")

			case act := <- ag.queue:
				count++
				fmt.Printf("%d %+v\n", count, act)
				if count >= 4 {
					return
				}

			}
		}
	})
}
