package tools

import (
	log "github.com/Sirupsen/logrus"

	"os/exec"
	"time"
	"strings"
	"os"
)

func Exec(env []string, timeout time.Duration, main string, cmds ...string) error {
	cmdName := main + " " + strings.Join(cmds, " ")
	log.WithField("cmd", cmdName).WithField("env", env).Debug("executing")

	done := make(chan struct{}, 1)
	cmd := exec.Command(main, cmds...)
	cmd.Env = env

	go func() {

		select {
		case <-done:
			return
		case <-time.After(timeout):
			cmd.Process.Kill()
			return
		}

	}()

	if log.GetLevel() == log.DebugLevel {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err := cmd.Start()
	if err != nil {
		done <- struct{}{}
		return err
	}

	err = cmd.Wait()
	done <- struct{}{}

	if err != nil {
		log.WithField("cmd", cmdName).WithField("err", err).WithField("env", env).Warn("exec failed")
	}

	return err
}
