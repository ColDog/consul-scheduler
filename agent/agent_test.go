package agent

import (
	. "github.com/coldog/scheduler/api"

	"testing"
	"time"
	"fmt"
)

func TestAgentSync(t *testing.T) {
	ag := NewAgent(NewSchedulerApi())

	task := Task{
		Host: ag.Host,
		Service: "test-service2",
		Cluster: Cluster{
			Name: "test-cluster2",
		},
		TaskDef: TaskDefinition{
			Name: "test2",
			Containers: []Container{
				Container{
					Executor: "bash",
					Bash: BashExecutor{
						Cmd: "echo",
						Args: []string{"hello there"},
					},
				},
			},
		},
	}

	ag.api.PutTask(task)
	task, ok := ag.api.GetTask(task.Id())
	if !ok {
		t.Fatal(task)
	}

	ag.api.DebugState("")
	ag.sync()

	go func() {
		time.Sleep(3 * time.Second)
		ag.quit <- struct {}{}
	}()

	ag.runner()


	c := ag.api.HealthyTaskCount(task.Name())
	fmt.Printf("count: %v\n", c)
}

func TestPublishAgent(t *testing.T) {
	ag := NewAgent(NewSchedulerApi())
	ag.publishState()
}
