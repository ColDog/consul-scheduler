package agent

import (
	log "github.com/Sirupsen/logrus"

	"fmt"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestAgent_GetsInfo(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		ag := NewAgent(a, &AgentConfig{})
		ag.GetHostName()

		ag.PublishState()

		fmt.Printf("%+v\n", ag.LastState)
	})
}

func TestAgent_Syncing(t *testing.T) {

	a := api.NewMockApi()

	ag := NewAgent(a, &AgentConfig{})
	ag.GetHostName()
	ag.PublishState()

	task := api.SampleTask()
	task.Host = ag.Host

	a.PutService(api.SampleService())
	a.PutCluster(api.SampleCluster())
	a.ScheduleTask(task)

	go func() {
		time.Sleep(1 * time.Second)
		ag.Stop()
	}()

	ag.Run()

	tools.Assert(t, len(ag.TaskState) > 0, "no tasks scheduled")
}
