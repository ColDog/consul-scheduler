package api

import (
	log "github.com/Sirupsen/logrus"

	"os/exec"
	"os"
	"syscall"
	"fmt"
)

func init()  {
	log.SetLevel(log.DebugLevel)
}

func NewConsulAgent() *TestConsulAgent {
	a := &TestConsulAgent{}
	a.Start()
	return a
}

type TestConsulAgent struct {
	cmd *exec.Cmd
}

func (a *TestConsulAgent) Start() {
	a.cmd = exec.Command("consul", "agent", "-dev", "-ui", "-bind=127.0.0.1")

	if os.Getenv("TEST_LOG_CONSUL") != "" {
		a.cmd.Stderr = os.Stderr
		a.cmd.Stdout = os.Stdout
	}

	err := a.cmd.Start()
	if err != nil {
		panic(err)
	}

}

func (a *TestConsulAgent) Stop() {
	a.cmd.Process.Signal(syscall.SIGTERM)
}

type ConsulApiTest func(api *ConsulApi)
func RunConsulApiTest(f ConsulApiTest)  {
	fmt.Println("\nbooting consul...")
	agent := NewConsulAgent()
	defer agent.Stop()

	api := newConsulApi()

	for {
		_, _, err := api.kv.Get("test", nil)
		if err == nil {
			break
		}
	}

	f(api)
}
