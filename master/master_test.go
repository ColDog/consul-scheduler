package master

import (
	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"fmt"
	"testing"
	"time"
	"github.com/coldog/sked/backends/mock"
)

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
