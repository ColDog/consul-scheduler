package tools

import (
	log "github.com/Sirupsen/logrus"

	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ExecErr struct {
	Output string
	Err    error
}

func (e *ExecErr) Error() string {
	return fmt.Sprintf("%s: %s", e.Err.Error(), e.Output)
}

func Exec(env []string, timeout DurationInterface, main string, cmds ...string) error {
	cmdName := main + " " + strings.Join(cmds, " ")
	log.WithField("cmd", cmdName).WithField("env", env).Debug("executing")

	done := make(chan struct{}, 1)
	cmd := exec.Command(main, cmds...)
	cmd.Env = env

	go func() {

		select {
		case <-done:
			return
		case <-time.After(time.Duration(timeout.Nanoseconds())):
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
			return
		}

	}()

	out, err := cmd.CombinedOutput()
	done <- struct{}{}

	if err != nil {
		log.WithField("cmd", cmdName).WithField("err", err).WithField("env", env).Warn("exec failed")
		return &ExecErr{string(out), err}
	}

	return nil
}
