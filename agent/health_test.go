package agent

import (
	"testing"
	"github.com/coldog/sked/api"
	"time"
	"github.com/coldog/sked/tools"
)

func TestHealth_HTTP(t *testing.T) {
	a := api.NewMockApi()

	m := NewMonitor(a, &api.Check{
		ID: "test-check",
		HTTP: "http://localhost:4121",
		Interval: 1 * time.Second,
		Timeout: 2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
	m.Stop()
}


func TestHealth_TCP(t *testing.T) {
	a := api.NewMockApi()

	m := NewMonitor(a, &api.Check{
		ID: "test-check",
		TCP: "localhost:4121",
		Interval: 1 * time.Second,
		Timeout: 2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
	m.Stop()
}

func TestHealth_Script(t *testing.T) {
	a := api.NewMockApi()

	m := NewMonitor(a, &api.Check{
		ID: "test-check",
		Script: "echo 'hello'",
		Interval: 1 * time.Second,
		Timeout: 2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "healthy", "check is not passing")
}

func TestHealth_Docker(t *testing.T) {
	a := api.NewMockApi()

	m := NewMonitor(a, &api.Check{
		ID: "test-check",
		Docker: "echo 'hello'",
		Interval: 1 * time.Second,
		Timeout: 2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
}
