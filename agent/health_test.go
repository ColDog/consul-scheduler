package agent

import (
	"testing"
	"github.com/coldog/sked/api"
	"time"
	"github.com/coldog/sked/tools"
)

func TestHealth(t *testing.T) {
	a := api.NewMockApi()

	m := NewMonitor(a, &api.Check{
		ID: "test-check",
		HTTP: "http://localhost:4121",
		Interval: 1 * time.Second,
		Timeout: 2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
}
