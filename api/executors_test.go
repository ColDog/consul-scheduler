package api

import (
	"testing"
	"fmt"
)

func TestDocker(t *testing.T) {
	d := DockerExecutor{
		Image: "ubuntu",
		Cmd: "bash",
		Env: []string{"test=stuff"},
	}

	fmt.Printf("cmds: %v\n", d.commands())
}

func TestBash(t *testing.T) {
	b := BashExecutor{
		Cmd: "stuff",
		Artifact: "http://stuff.com",
		DownloadDir: "/usr/local/stuff",
		Env: []string{"stuff=stuff"},
		Args: []string{"-a", "abc"},
	}

	fmt.Printf("cmds: %v\n", b.commands())

}