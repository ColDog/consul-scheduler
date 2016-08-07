package scheduler

import (
	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"

	"testing"
	"fmt"
	"time"
)

func init()  {
	log.SetLevel(log.DebugLevel)
}

func seed(api *SchedulerApi) Cluster {
	c := Cluster{
		Name: "default",
		Scheduler: "default",
		Services: []string{"test-service"},
	}

	s := Service{
		Name: "test-service",
		TaskName: "web-test2",
		TaskVersion: 5,
		Desired: 5,
		Min: 1,
		Max: 7,
	}

	t1 := TaskDefinition{
		Name: "test",
		Version: 1,
		Memory: 300,
		CpuUnits: 1,
	}

	t2 := TaskDefinition{
		Name: "web-test2",
		Version: 5,
		Memory: 300,
		CpuUnits: 1,
		Containers: []Container{
			{
				Name: "web",
				Executor: "bash",
				Bash: BashExecutor{
					Cmd: "echo",
					Args: []string{"hello"},
				},
			},
		},
		Checks: []Check{
			{Http: "127.0.0.1:80", Interval: "20s"},
		},
	}

	api.PutCluster(c)
	api.PutService(s)
	api.PutTaskDefinition(t1)
	api.PutTaskDefinition(t2)

	return c
}

func TestScheduler(t *testing.T) {

	a := NewSchedulerApi()

	c := seed(a)

	a.PutHost(Host{
		Name: a.Host(),
		CpuUnits: 1,
		Memory: 1,
	})

	a.DebugKeys()

	DefaultScheduler(c, a)

	fmt.Println("post scheduling")
	a.DebugKeys()
}

func TestRunningMaster(t *testing.T) {

	a := NewSchedulerApi()
	a.FinishedScheduling()
	m := NewMaster(a)

	go m.Run()

	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("starting scheduler")
		a.TriggerScheduler()
	}()

	time.Sleep(5 * time.Second)
}

//func TestFull(t *testing.T) {
//	a := NewSchedulerApi()
//
//	seed(a)
//
//	a.FinishedScheduling()
//
//	m := NewMaster(a)
//	ag := NewAgent(a)
//
//	go m.Run()
//	go ag.Run()
//
//	go func() {
//		time.Sleep(2 * time.Second)
//		fmt.Println("starting scheduler")
//		a.TriggerScheduler()
//	}()
//
//	time.Sleep(30 * time.Second)
//}