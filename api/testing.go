package api

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"os"
	"os/exec"
	"syscall"
	"time"
)

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

func RunConsulApiTest(f ConsulApiTest) {
	agent := NewConsulAgent()
	defer agent.Stop()

	fmt.Println("--- starting consul")

	api := NewConsulApi(DefaultStorageConfig(), api.DefaultConfig())

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

func SampleContainer() *Container {
	return &Container{
		Name:     "test",
		Type:     "bash",
		Executor: []byte(`{"start": ["echo start"], "stop": ["echo stop"]}`),
	}
}

func SampleTaskDefinition() *TaskDefinition {
	return &TaskDefinition{
		Name:        "test",
		Version:     1,
		ProvidePort: true,
		Tags:        []string{"test"},
		Containers: []*Container{
			SampleContainer(),
		},
	}
}

func SampleService() *Service {
	return &Service{
		Name:        "test",
		TaskName:    "test",
		TaskVersion: 0,
		Desired:     1,
		Max:         1,
	}
}

func SampleHost() *Host {
	return &Host{
		Name:          "testinghost",
		CpuUnits:      10000000,
		MemUsePercent: 0.60,
		Memory:        100000,
		DiskSpace:     1000000,
		ReservedPorts: []uint{1000, 1001, 1002, 1003, 1004, 1005, 1006},
		PortSelection: []uint{2000, 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010},
	}
}

func SampleTask() *Task {
	return NewTask(SampleCluster(), SampleTaskDefinition(), SampleService(), 1)
}

func SampleCluster() *Cluster {
	return &Cluster{
		Name:      "test",
		Services:  []string{"test"},
	}
}
