package agent

import (
	log "github.com/Sirupsen/logrus"

	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"fmt"
	"testing"
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

func TestAgent_Start(t *testing.T) {
	tk := api.SampleTask()

	a := api.NewMockApi()
	ag := NewAgent(a, &AgentConfig{})

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
	ag := NewAgent(a, &AgentConfig{})
	ag.Host = "local"


	ag.PublishState()

	h, err := a.GetHost("local")
	tools.Ok(t, err)
	tools.Equals(t, h.Name, "local")
}


