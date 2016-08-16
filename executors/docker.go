package executors

import (
	. "github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/tools"

	"fmt"
)

// The docker executor will start a docker container.
type DockerExecutor struct {
	Image         string   `json:"image"`
	Name          string   `json:"name"`
	Cmd           string   `json:"cmd"`
	Entry         string   `json:"entry"`
	ContainerPort uint     `json:"container_port"`
	Ports         []string `json:"ports"`
	Env           []string `json:"env"`
	WorkDir       string   `json:"work_dir"`
	Volumes       []string `json:"volumes"`
	Flags         []string `json:"flags"`
}

func (docker DockerExecutor) StartTask(t Task) error {
	docker.Name = t.Id()

	if t.TaskDef.ProvidePort {
		p := docker.ContainerPort
		if p == uint(0) {
			p = t.Port
		}
		docker.Ports = append(docker.Ports, fmt.Sprintf("%d:%d", t.Port, p))
	}

	err := tools.Exec(docker.Env, "docker", "pull", docker.Image)
	if err != nil {
		return err
	}

	tools.Exec(docker.Env, "docker", "rm", "-f", docker.Name)

	main := []string{"run"}

	if docker.Name != "" {
		main = append(main, "--name", docker.Name)
	}

	if docker.Entry != "" {
		main = append(main, "--entrypoint", docker.Entry)
	}

	if docker.WorkDir != "" {
		main = append(main, "-w", docker.WorkDir)
	}

	main = append(main, docker.Flags...)

	for _, v := range docker.Volumes {
		main = append(main, "-v", v)
	}

	for _, e := range docker.Env {
		main = append(main, "-e", e)
	}

	for _, p := range docker.Ports {
		main = append(main, "-p", p)
	}

	main = append(main, "-d", docker.Image)
	return tools.Exec(docker.Env, "docker", main...)
}

func (docker DockerExecutor) StopTask(t Task) error {
	return tools.Exec(docker.Env, "docker", "stop", t.Id())
}
