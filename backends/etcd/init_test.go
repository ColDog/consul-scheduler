package etcd

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/coldog/sked/tools"
	"github.com/coreos/etcd/client"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type EtcdApiTest func(api *EtcdApi)

func RunEtcdAPITest(f EtcdApiTest) {
	agent := NewEtcd()
	defer agent.Stop()

	fmt.Println("--- starting etcd")

	api := NewEtcdApi(&Config{
		Prefix:  "/registry/",
		LockTTL: tools.Duration{30 * time.Second},
		ClientConfig: &client.Config{
			Endpoints: []string{"http://127.0.0.1:2379"},
			Transport: client.DefaultTransport,
			// set timeout per request to fail fast when the target endpoint is unavailable
			HeaderTimeoutPerRequest: time.Second,
		}},
	)

	time.Sleep(1 * time.Second)

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
	a.cmd = exec.Command("etcd", "-data-dir=~/documents/default.etcd")

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
	out, _ := exec.Command("rm", "-rf", "~/documents/default.etcd").CombinedOutput()
	fmt.Println(string(out))
}
