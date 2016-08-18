package api

import (
	"testing"
	"github.com/coldog/scheduler/tools"
)

func TestExecutors_Bash(t *testing.T) {
	b := BashExecutor{
		Start: []string{"echo hello"},
		Stop: []string{"echo stop"},
	}

	task := sampleTask()
	err := b.StartTask(task)
	tools.Ok(t, err)

	err = b.StopTask(task)
	tools.Ok(t, err)
}

func TestExecutors_Docker(t *testing.T) {
	b := DockerExecutor{
		Image: "ubuntu",
		ContainerPort: 8080,
		Env: []string{"HI=hello"},

	}

	task := sampleTask()
	err := b.StartTask(task)
	tools.Ok(t, err)

	err = b.StopTask(task)
	tools.Ok(t, err)
}
