package api

import (
	log "github.com/Sirupsen/logrus"

	"os/exec"
	"os"
	"syscall"
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
	a.cmd.Stderr = os.Stderr
	a.cmd.Stdout = os.Stdout

	err := a.cmd.Start()
	if err != nil {
		panic(err)
	}

}

func (a *TestConsulAgent) Stop() {
	a.cmd.Process.Signal(syscall.SIGTERM)
}
