package api

import (
	"github.com/coldog/sked/tools"

	"encoding/json"
	"fmt"
	"time"
	"strings"
)

// a runnable task description
// validations:
// 	- version must be new
//
type TaskDefinition struct {
	Name        string         `json:"name"`
	Version     uint           `json:"version"`
	Tags        []string       `json:"tags"`
	Containers  []*Container   `json:"containers"`
	GracePeriod tools.Duration `json:"grace_period"`
}

// all the ports this task needs to run
func (t *TaskDefinition) AllPorts() []uint {
	ports := []uint{}

	for _, c := range t.Containers {
		for _, p := range c.Ports {
			if p.Host != 0 {
				ports = append(ports, p.Host)
			}
		}
	}

	return ports
}

type Counts struct {
	Memory   int64 `json:"memory"`
	CpuUnits int64 `json:"cpu_units"`
	DiskUse  int64 `json:"disk_use"`
}

func (t *TaskDefinition) Counts() (counts Counts) {
	for _, c := range t.Containers {
		counts.DiskUse += c.DiskUse
		counts.CpuUnits += c.CpuUnits
		counts.Memory += c.Memory
	}

	return
}

func (task *TaskDefinition) Validate(api SchedulerApi) (errors []string) {
	_, err := api.GetTaskDefinition(task.Name, task.Version)
	if err == nil {
		errors = append(errors, "version already provisioned")
	}

	if task.GracePeriod.IsNone() {
		task.GracePeriod.Duration = 60 * time.Second
	}

	for _, c := range task.Containers {
		for i, check := range c.Checks {
			check.ID = fmt.Sprintf("%s_%s-%s-%d", task.Name, c.Name, check.Name, i)

			if check.Interval.IsNone() {
				check.Interval.Duration = 30 * time.Second
			}

			if check.Timeout.IsNone() {
				check.Timeout.Duration = 5 * time.Second
			}

			if check.HTTP == "" && check.TCP == "" && check.Script == "" && check.Docker == "" {
				errors = append(errors, "health check malformed")
			}
		}
	}

	return errors
}

func (t *TaskDefinition) Key() string {
	return "config/task/" + t.Name
}

type Container struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Executor json.RawMessage `json:"executor"`
	Setup    []string        `json:"setup"`
	Teardown []string        `json:"teardown"`
	Checks   []*Check        `json:"checks"`
	Ports    []*PortMapping  `json:"ports"`
	Memory   int64           `json:"memory"`
	CpuUnits int64           `json:"cpu_units"`
	DiskUse  int64           `json:"disk_use"`
	Essential bool           `json:"essential"`

	bash   *BashExecutor
	docker *DockerExecutor
}

func (c *Container) RunSetup() error {
	for _, cmd := range c.Setup {
		err := tools.Exec(nil, 20*time.Second, "sh", "-c", cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Container) RunTeardown() error {
	for _, cmd := range c.Setup {
		err := tools.Exec(nil, 20*time.Second, "sh", "-c", cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// given port mappings for a task
func (c *Container) PortsForTask(t *Task) []*PortMapping {
	mappings := make([]*PortMapping, len(c.Ports))

	for _, p := range c.Ports {
		host := p.Host
		if host == 0 {
			host = t.ProvidedPorts[p.Name]
		}

		mappings = append(mappings, &PortMapping{
			Host: host,
			Container: p.Container,
			Name: p.Name,
		})
	}

	return mappings
}

// a check passed along to consul
type Check struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	HTTP     string         `json:"http"`
	TCP      string         `json:"tcp"`
	Script   string         `json:"script"`
	Interval tools.Duration `json:"interval"`
	Timeout  tools.Duration `json:"timeout"`
	Docker   string         `json:"docker"`
}

func (ch *Check) HTTPWithPort(t *Task, c *Container) string {
	u := ch.HTTP
	for _, p := range c.PortsForTask(t) {
		u = strings.Replace(u, "$" + p.Name, fmt.Sprintf("%s", p.Host), 1)
	}
	return u
}

func (ch *Check) TCPWithPort(t *Task, c *Container) string {
	u := ch.TCP
	for _, p := range c.PortsForTask(t) {
		u = strings.Replace(u, "$" + p.Name, fmt.Sprintf("%s", p.Host), 1)
	}
	return u
}

type PortMapping struct {
	Name      string `json:"name"`
	Container uint   `json:"container"`
	Host      uint   `json:"host"`
}
