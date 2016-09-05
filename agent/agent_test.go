package agent

import (
	log "github.com/Sirupsen/logrus"

	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"
	"github.com/coldog/sked/config"

	"fmt"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func testConfig() *AgentConfig {
	return &AgentConfig{
		AppConfig: config.NewConfig(),
		Resources: &api.Resources{},
	}
}

func queueSync(ag *Agent) (list []action) {
	for {
		select {
		case a := <-ag.queue:
			list = append(list, a)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
	return list
}

func TestAgent_GetsInfo(t *testing.T) {
	api.RunConsulApiTest(func(a *api.ConsulApi) {
		ag := NewAgent(a, testConfig())
		ag.GetHostName()

		ag.PublishState()

		fmt.Printf("%+v\n", ag.LastState)
	})
}

func TestAgent_Start(t *testing.T) {
	tk := api.SampleTask()

	a := api.NewMockApi()
	ag := NewAgent(a, testConfig())

	err := ag.start(tk)
	tools.Ok(t, err)

	st := ag.TaskState.get(tk.Id())
	tools.Equals(t, st.Attempts, 1)
	tools.Equals(t, st.Healthy, false)
	tools.Equals(t, st.Failure, nil)
}

func TestAgent_Stop(t *testing.T) {
	tk := api.SampleTask()

	a := api.NewMockApi()
	ag := NewAgent(a, &AgentConfig{})

	err := ag.stop(tk)
	tools.Ok(t, err)

	_, err = a.GetTask(tk.Id())
	tools.Assert(t, err == api.ErrNotFound, "task was found")
}

func TestAgent_PublishState(t *testing.T) {
	a := api.NewMockApi()
	ag := NewAgent(a, testConfig())
	ag.Host = "local"

	ag.PublishState()

	h, err := a.GetHost("local")
	tools.Ok(t, err)
	tools.Equals(t, h.Name, "local")
}

func TestAgent_Sync(t *testing.T) {
	a := api.NewMockApi()
	ag := NewAgent(a, testConfig())
	ag.Host = "local"

	for i := 0; i < 20; i++ {
		tk := api.SampleTask()
		tk.Instance = i
		tk.Scheduled = true
		tk.Host = "local"
		tk.Port = uint(10000 + i) // no conflicts
		tk.TaskDefinition.ProvidePort = true
		a.PutTask(tk)
	}

	fmt.Println("-> sync 1")
	ag.sync()
	list := queueSync(ag)
	tools.Equals(t, 20, len(list))

	for _, a := range list {
		tools.Equals(t, a.start, true)
	}

	tasks, _ := a.ListTasks(&api.TaskQueryOpts{ByHost: "local"})
	for _, tk := range tasks {
		tk.Scheduled = false
		a.PutTask(tk)
	}
	tools.Equals(t, len(tasks), 20)

	fmt.Println("-> sync 2")
	ag.sync()
	list2 := queueSync(ag)
	tools.Equals(t, 20, len(list2))

	for _, a := range list2 {
		tools.Equals(t, a.stop, true)
	}
}
