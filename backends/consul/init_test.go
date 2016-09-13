package consul

import (
	consul "github.com/hashicorp/consul/api"

	"fmt"
	"time"
	"os/exec"
	"os"
	"syscall"
)

var consulBinary = "consul"

func init() {
	if os.Getenv("CONSUL_BINARY") != "" {
		consulBinary = os.Getenv("CONSUL_BINARY")
	}
}

type ConsulApiTest func(api *ConsulApi)

func RunConsulApiTest(f ConsulApiTest) {
	agent := NewConsulAgent()
	defer agent.Stop()

	fmt.Println("--- starting consul")

	api := NewConsulApi("registry", consul.DefaultConfig())

	for {
		_, _, err := api.kv.Get("test", nil)
		if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("--- begin test")
	f(api)
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
	a.cmd = exec.Command(consulBinary, "agent", "-dev", "-ui", "-bind=127.0.0.1")

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
