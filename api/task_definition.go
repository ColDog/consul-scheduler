package api

import (
	"github.com/coldog/sked/tools"

	"encoding/json"
	"time"
	"fmt"
)

// a runnable task description
// validations:
// 	- version must be new
//
type TaskDefinition struct {
	Name        string        `json:"name"`
	Version     uint          `json:"version"`
	ProvidePort bool          `json:"provide_port"`
	Port        uint          `json:"port"`
	Tags        []string      `json:"tags"`
	Containers  []*Container  `json:"containers"`
	GracePeriod time.Duration `json:"grace_period"`
	MaxAttempts int           `json:"max_attempts"`
}

// all the ports this task needs to run
func (t *TaskDefinition) AllPorts() []uint {
	ports := []uint{}
	if t.Port != 0 {
		ports = append(ports, t.Port)
	}

	for _, c := range t.Containers {
		ex := c.GetExecutor()
		if ex != nil {
			ports = append(ports, ex.ReservedPorts()...)
		}
	}

	return ports
}

type Counts struct {
	Memory   uint64 `json:"memory"`
	CpuUnits uint64 `json:"cpu_units"`
	DiskUse  uint64 `json:"disk_use"`
}

func (t *TaskDefinition) Counts() Counts {
	c := Counts{}

	for _, c := range t.Containers {
		c.DiskUse += c.DiskUse
		c.CpuUnits += c.CpuUnits
		c.Memory += c.Memory
	}

	return c
}

func (task *TaskDefinition) Validate(api SchedulerApi) (errors []string) {
	_, err := api.GetTaskDefinition(task.Name, task.Version)
	if err == nil {
		errors = append(errors, "version already provisioned")
	}

	if task.MaxAttempts == 0 {
		task.MaxAttempts = 10
	}

	if task.GracePeriod == 0 {
		task.GracePeriod = 60 * time.Second
	}

	for _, c := range task.Containers {
		for i, check := range c.Checks {
			check.ID = fmt.Sprintf("%s_%s-%s-%d", task.Name, c.Name, check.Name, i)

			if check.Interval == 0 {
				check.Interval = 30 * time.Second
			}

			if check.Timeout == 0 {
				check.Timeout = 5 * time.Second
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
	Memory   uint64          `json:"memory"`
	CpuUnits uint64          `json:"cpu_units"`
	DiskUse  uint64          `json:"disk_use"`
	TaskID   string          `json:"task_id"`
	bash     *BashExecutor
	docker   *DockerExecutor
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

// a check passed along to consul
type Check struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	HTTP     string        `json:"http"`
	TCP      string        `json:"tcp"`
	Script   string        `json:"script"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	TTL      string        `json:"ttl"`
	Docker   string        `json:"docker"`
	TaskID   string        `json:"task_id"`
}
