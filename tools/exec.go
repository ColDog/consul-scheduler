package tools

import (
	log "github.com/Sirupsen/logrus"

	"os/exec"
	"time"
	"os"
	"strings"
)

func Exec(env []string, main string, cmds ...string) error {
	if log.GetLevel() == log.DebugLevel {
		cmdName := main + " " + strings.Join(cmds, " ")
		log.WithField("cmd", cmdName).WithField("env", env).Debug("executing")
	}

	done := make(chan struct{}, 1)
	cmd := exec.Command(main, cmds...)
	cmd.Env = env

	go func() {

		select {
		case <-done:
			return
		case <-time.After(30 * time.Second):
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
	return err
}
