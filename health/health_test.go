package health

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/backends/mock"
	"github.com/coldog/sked/tools"

	"testing"
	"time"
)

var task = api.SampleTask()

func TestHealth_HTTP(t *testing.T) {
	a := mock.NewMockApi()

	m := NewMonitor(a, &HealthCheck{
		TaskID:   "test-check",
		HTTP:     "http://localhost:4121",
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
	m.Stop()
}

func TestHealth_TCP(t *testing.T) {
	a := mock.NewMockApi()

	m := NewMonitor(a, &HealthCheck{
		TaskID:   "test-check",
		TCP:      "localhost:4121",
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
	m.Stop()
}

func TestHealth_Script(t *testing.T) {
	a := mock.NewMockApi()

	m := NewMonitor(a, &HealthCheck{
		TaskID:   "test-check",
		Script:   "echo 'hello'",
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "healthy", "check is not passing")
}

func TestHealth_Docker(t *testing.T) {
	a := mock.NewMockApi()

	m := NewMonitor(a, &HealthCheck{
		TaskID:   "test-check",
		Docker:   "echo 'hi'",
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})

	time.Sleep(3 * time.Second)
	tools.Assert(t, m.Status == "warning", "check is passing")
}
