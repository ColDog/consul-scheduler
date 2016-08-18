package api

import (
	"encoding/json"
	"fmt"
	"github.com/coldog/scheduler/tools"
)

func GetExecutor(c *Container) Executor {

	if c.Type == "docker" {
		res := DockerExecutor{}
		err := json.Unmarshal(c.Executor, &res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}
		return res
	} else if c.Type == "bash" {
		res := BashExecutor{}
		err := json.Unmarshal(c.Executor, &res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}
		return res
	}

	return nil
}


// The bash executor simply runs a list of commands to start the process, and then runs another
// list of commands to stop the process. This is a very rough executor as it doesn't know much
// about the underlying process and therefore cannot intelligently make many decisions about it.
type BashExecutor struct {
	Start       []string `json:"start"`
	Stop        []string `json:"stop"`
	Env         []string `json:"env"`
	Artifact    string   `json:"artifact"`
	DownloadDir string   `json:"download_dir"`
}

func (bash BashExecutor) StartTask(t *Task) error {
	if t.TaskDefinition.ProvidePort {
		bash.Env = append(bash.Env, fmt.Sprintf("SCHEDULED_PORT=%d", t.Port))
	}

	if bash.Artifact != "" {
		err := tools.Exec(bash.Env, "curl", "-o", bash.DownloadDir, bash.Artifact)
		if err != nil {
			return err
		}
	}

	for _, cmd := range bash.Start {
		err := tools.Exec(bash.Env, "/bin/bash", "-c", cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bash BashExecutor) StopTask(t *Task) (err error) {
	for _, cmd := range bash.Start {
		err = tools.Exec(bash.Env, "/bin/bash", "-c", cmd)
	}
	return err
}


// The docker executor will start a docker container.
type DockerExecutor struct {
	Image         string   `json:"image"`
	Cmd           string   `json:"cmd"`
	Entry         string   `json:"entry"`
	ContainerPort uint     `json:"container_port"`
	Ports         []string `json:"ports"`
	Env           []string `json:"env"`
	WorkDir       string   `json:"work_dir"`
	Volumes       []string `json:"volumes"`
	Flags         []string `json:"flags"`
}

func (docker DockerExecutor) StartTask(t *Task) error {
	if t.TaskDefinition.ProvidePort {
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

	tools.Exec(docker.Env, "docker", "rm", "-f", t.Id())

	main := []string{"run"}
	main = append(main, "--name", t.Id())

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

func (docker DockerExecutor) StopTask(t *Task) error {
	return tools.Exec(docker.Env, "docker", "stop", t.Id())
}
