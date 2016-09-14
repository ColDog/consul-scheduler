package etcd

import (
	"fmt"
	"time"
	"os/exec"
	"os"
	"syscall"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

type EtcdApiTest func(api *EtcdApi)

func RunEtcdAPITest(f EtcdApiTest) {
	agent := NewEtcd()
	defer agent.Stop()

	fmt.Println("--- starting etcd")

	api := NewEtcdApi("/registry/", client.Config{
		Endpoints:               []string{"http://127.0.0.1:2379"},
		Transport:               client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	})

	for {
		_, err := api.kv.Get(context.Background(), "test", nil)
		if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Println("--- begin test")
	f(api)
}

func NewEtcd() *TestEtcd {
	a := &TestEtcd{}
	a.Start()
	return a
}


type TestEtcd struct {
	cmd *exec.Cmd
}

func (a *TestEtcd) Start() {
	a.cmd = exec.Command("etcd")

	if os.Getenv("TEST_LOG_ETCD") != "" {
		a.cmd.Stderr = os.Stderr
		a.cmd.Stdout = os.Stdout
	}

	err := a.cmd.Start()
	if err != nil {
		panic(err)
	}

}

func (a *TestEtcd) Stop() {
	a.cmd.Process.Signal(syscall.SIGTERM)
}

