package master

import (
	"testing"
	"github.com/coldog/sked/api"
	"time"
	"github.com/coldog/sked/actions"
	"fmt"
	"github.com/coldog/sked/tools"
)

func TestMaster_WillSchedule(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		m := NewMaster(a, &Config{Runners: 2})


		go func() {
			time.Sleep(1 * time.Second)
			m.dispatch()
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
	})
}
