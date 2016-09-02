package api

import (
	"encoding/json"
	"fmt"
	"github.com/coldog/sked/tools"
	"strconv"
	"strings"
	"time"
)

type ExecutorBuilder func(c *Container) Executor
var ExecutorBuilders = make(map[string]ExecutorBuilder)

func UseExecutor(name string, b ExecutorBuilder)  {
	ExecutorBuilders[name] = b
}

func (c *Container) GetExecutor() Executor {

	if c.Type == "docker" {
		if c.docker != nil {
			return c.docker
		}

		res := &DockerExecutor{}
		err := json.Unmarshal(c.Executor, res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}

		c.docker = res
		return res
	} else if c.Type == "bash" {
		if c.bash != nil {
			return c.bash
		}

		res := &BashExecutor{}
		err := json.Unmarshal(c.Executor, res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}

		c.bash = res
		return res
	}

	return nil
}

// The bash executor simply runs a list of commands to start the process, and then runs another
// list of commands to stop the process. This is a very rough executor as it doesn't know much
// about the underlying process and therefore cannot intelligently make many decisions about it.
type BashExecutor struct {
	Ports            []uint        `json:"ports"`
	Start            []string      `json:"start"`
	Stop             []string      `json:"stop"`
	Env              []string      `json:"env"`
	Artifact         string        `json:"artifact"`
	DownloadDir      string        `json:"download_dir"`
	AllowedStartTime time.Duration `json:"allowed_start_time"`
}

func (bash *BashExecutor) StartTask(t *Task) error {
	if t.TaskDefinition.ProvidePort {
		bash.Env = append(bash.Env, fmt.Sprintf("SCHEDULED_PORT=%d", t.Port))
	}

	if bash.Artifact != "" {
		err := tools.Exec(bash.Env, 30*time.Second, "curl", "-o", bash.DownloadDir, bash.Artifact)
		if err != nil {
			return err
		}
	}

	for _, cmd := range bash.Start {
		err := tools.Exec(bash.Env, bash.AllowedStartTime, "sh", "-c", cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bash *BashExecutor) StopTask(t *Task) (err error) {
	for _, cmd := range bash.Start {
		err = tools.Exec(bash.Env, 20*time.Second, "sh", "-c", cmd)
	}
	return err
}

func (bash *BashExecutor) ReservedPorts() []uint {
	return bash.Ports
}

// The docker executor will start a docker container.
type DockerExecutor struct {
	Image            string        `json:"image"`
	Cmd              string        `json:"cmd"`
	Entry            string        `json:"entry"`
	ContainerPort    uint          `json:"container_port"`
	Ports            []string      `json:"ports"`
	Env              []string      `json:"env"`
	WorkDir          string        `json:"work_dir"`
	Net              string        `json:"net"`
	NetAlias         string        `json:"net_alias"`
	Volumes          []string      `json:"volumes"`
	VolumeDriver     string        `json:"volume_driver"`
	Flags            []string      `json:"flags"`
	AllowedStartTime time.Duration `json:"allowed_start_time"`
}

func (docker *DockerExecutor) StartTask(t *Task) error {
	if t.TaskDefinition.ProvidePort {
		p := docker.ContainerPort
		if p == uint(0) {
			p = t.Port
		}
		docker.Ports = append(docker.Ports, fmt.Sprintf("%d:%d", t.Port, p))
	}

	err := tools.Exec(docker.Env, 3*time.Minute, "docker", "pull", docker.Image)
	if err != nil {
		return err
	}

	tools.Exec(docker.Env, 5*time.Second, "docker", "rm", "-f", t.Id())

	main := []string{"run"}
	main = append(main, "--name", t.Id())

	if docker.Entry != "" {
		main = append(main, "--entrypoint", docker.Entry)
	}

	if docker.WorkDir != "" {
		main = append(main, "-w", docker.WorkDir)
	}

	if docker.VolumeDriver != "" {
		main = append(main, "--volume-driver", docker.VolumeDriver)
	}

	if docker.NetAlias != "" {
		main = append(main, "--net-alias", docker.NetAlias)
	}

	if docker.Net != "" {
		main = append(main, "--net", docker.Net)
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
	return tools.Exec(docker.Env, docker.AllowedStartTime, "docker", main...)
}

func (docker *DockerExecutor) StopTask(t *Task) error {
	return tools.Exec(docker.Env, 20*time.Second, "docker", "stop", t.Id())
}

func (docker *DockerExecutor) ReservedPorts() []uint {
	ps := []uint{}
	for _, p := range docker.Ports {
		port := strings.Split(p, ":")[0]
		i, _ := strconv.ParseInt(port, 8, 64)
		ps = append(ps, uint(i))
	}
	return ps
}
