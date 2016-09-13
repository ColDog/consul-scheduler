package api

import (
	"github.com/coldog/sked/tools"

	log "github.com/Sirupsen/logrus"

	"encoding/json"
	"fmt"
	"time"
)

const (
	executorDownloadTimeout   = 30 * time.Second
	executorStopTaskTimeout   = 20 * time.Second
	executorDockerPullTimeout = 15 * time.Minute
)

type ExecutorBuilder func(c *Container) Executor

var ExecutorBuilders = map[string]ExecutorBuilder{
	"docker": func(c *Container) Executor {
		if c.docker != nil {
			return c.docker
		}

		res := &DockerExecutor{}
		err := json.Unmarshal(c.Executor, res)
		if err != nil {
			log.WithField("error", err).WithField("raw", string(c.Executor)).Warn("[docker-executor] cannot unmarshal executor")
			return nil
		}

		c.docker = res
		return res
	},
	"bash": func(c *Container) Executor {
		if c.bash != nil {
			return c.bash
		}

		res := &BashExecutor{}
		err := json.Unmarshal(c.Executor, res)
		if err != nil {
			log.WithField("error", err).WithField("raw", string(c.Executor)).Warn("[bash-executor] cannot unmarshal executor")
			return nil
		}

		c.bash = res
		return res
	},
}

func UseExecutor(name string, b ExecutorBuilder) {
	ExecutorBuilders[name] = b
}

func (c *Container) GetExecutor() Executor {
	if builder, ok := ExecutorBuilders[c.Type]; ok {
		return builder(c)
	}

	return nil
}

// The bash executor simply runs a list of commands to start the process, and then runs another
// list of commands to stop the process. This is a very rough executor as it doesn't know much
// about the underlying process and therefore cannot intelligently make many decisions about it.
type BashExecutor struct {
	Ports       []uint   `json:"ports"`
	Env         []string `json:"env"`
	Start       string   `json:"start"`
	Stop        string   `json:"stop"`
	Artifact    string   `json:"artifact"`
	DownloadDir string   `json:"download_dir"`
}

func (bash *BashExecutor) StartTask(t *Task, cont *Container) error {
	if bash.Artifact != "" {
		err := tools.Exec(bash.Env, executorDownloadTimeout, "curl", "-o", bash.DownloadDir, bash.Artifact)
		if err != nil {
			return err
		}
	}

	return tools.Exec(bash.Env, t.TaskDefinition.GracePeriod, "sh", "-c", bash.Start)
}

func (bash *BashExecutor) StopTask(t *Task, cont *Container) (err error) {
	return tools.Exec(bash.Env, executorStopTaskTimeout, "sh", "-c", bash.Stop)
}

func (bash *BashExecutor) IsRunning(t *Task, cont *Container) (bool, error) {
	return false, fmt.Errorf("cannot determine state")
}

// The docker executor will start a docker container.
type DockerExecutor struct {
	Image         string   `json:"image"`
	Cmd           string   `json:"cmd"`
	Entry         string   `json:"entry"`
	Env           []string `json:"env"`
	WorkDir       string   `json:"work_dir"`
	Net           string   `json:"net"`
	NetAlias      string   `json:"net_alias"`
	Volumes       []string `json:"volumes"`
	VolumeDriver  string   `json:"volume_driver"`
	Flags         []string `json:"flags"`
}

func (docker *DockerExecutor) StartTask(t *Task, cont *Container) error {
	err := tools.Exec(docker.Env, executorDockerPullTimeout, "docker", "pull", docker.Image)
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

	for _, p := range cont.PortsForTask(t) {
		main = append(main, "-p", fmt.Sprintf("%d:%d", p.Host, p.Container))
	}

	main = append(main, "-d", docker.Image)
	return tools.Exec(docker.Env, t.TaskDefinition.GracePeriod, "docker", main...)
}

func (docker *DockerExecutor) StopTask(t *Task, cont *Container) error {
	return tools.Exec(docker.Env, executorStopTaskTimeout, "docker", "stop", t.Id())
}

func (docker *DockerExecutor) IsRunning(t *Task, cont *Container) (bool, error) {
	err := tools.Exec(nil, executorStopTaskTimeout, "docker", "inspect", "-f", "{{.State.Running}}", t.ID())
	return err == nil, nil
}
