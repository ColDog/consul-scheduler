package api

import (
	"testing"
	"time"
	"fmt"
)

func TestGetDesiredHosts(t *testing.T) {
	a := NewSchedulerApi()

	a.PutTask(Task{
		Host: "test",
		Service: "test-service",
		Cluster: Cluster{
			Name: "test-cluster1",
		},
	})

	l := a.DesiredTasksByHost("test")

	if len(l) == 0 {
		t.Fatal(l)
	}
}

func TestRegisterTask(t *testing.T) {
	a := NewSchedulerApi()

	task := Task{
		Host: "test",
		Service: "test-service",
		Cluster: Cluster{
			Name: "test-cluster",
		},
		TaskDef: TaskDefinition{
			Name: "test",
			Containers: []Container{
				Container{
					Executor: "docker",
					Docker: DockerExecutor{
						Image: "ubuntu",
					},
				},
			},
			Checks: []Check{
				Check{
					Http: "http://127.0.0.1:80",
					Interval: "30s",
				},
			},
		},
	}

	a.PutTask(task)
	task, ok := a.GetTask(task.Id())
	if !ok {
		t.Fatal(task)
	}

	a.Register(task)

	tasks := a.RunningTasksOnHost("test")
	if _, ok := tasks[task.Id()]; !ok {
		t.Fatal(task)
	}


}

func TestBlockingScheduler(t *testing.T) {
	a := NewSchedulerApi()

	go func() {
		time.Sleep(2 * time.Second)
		a.TriggerScheduler()
	}()
	fmt.Printf("\nwaiting for scheduler... ")
	a.WaitForScheduler()
	a.FinishedScheduling()

	fmt.Printf("DONE!\n")
}