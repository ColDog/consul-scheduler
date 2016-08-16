package executors

import (
	. "github.com/coldog/scheduler/api"

	"fmt"
	"testing"
)

func TestDocker(t *testing.T) {
	d := DockerExecutor{
		Image: "ubuntu",
		Cmd:   "bash",
		Env:   []string{"test=stuff"},
	}

	d.StartTask()
	d.StopTask()
}

func TestBash(t *testing.T) {
	b := BashExecutor{
		Start:       "stuff",
		Artifact:    "http://stuff.com",
		DownloadDir: "/usr/local/stuff",
		Env:         []string{"stuff=stuff"},
	}

	fmt.Printf("cmds: %v\n", b.commands())

}
