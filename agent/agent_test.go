package agent

import (
	. "github.com/coldog/scheduler/api"
	. "github.com/coldog/scheduler/tools"

	"fmt"
	"testing"
	"time"
)

func TestAgentRunner(t *testing.T) {
	ag := NewAgent(NewSchedulerApi())
	defer ag.Stop()

	go ag.runner(0)


}

func TestAgentSync(t *testing.T) {
	ag := NewAgent(NewSchedulerApi())

	task := Task{
		Host:    ag.Host,
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
						Cmd:  "echo",
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
		ag.quit <- struct{}{}
	}()

	ag.runner()

	c := ag.api.HealthyTaskCount(task.Name())
	fmt.Printf("count: %v\n", c)
}

func TestPublishAgent(t *testing.T) {
	ag := NewAgent(NewSchedulerApi())
	ag.publishState()
}

//func TestStart(t *testing.T) {
//	ag := NewAgent(NewSchedulerApi())
//}
