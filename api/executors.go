package api

import "fmt"

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	Start(t Task) error
	Stop(t Task) error
}

// a type of executor
type BashExecutor struct {
	Start       []string `json:"start"`
	Stop        []string `json:"stop"`
	Env         []string `json:"env"`
	Artifact    string   `json:"artifact"`
	DownloadDir string   `json:"download_dir"`
}

func (bash BashExecutor) StartCmds(t Task) (cmds [][]string) {
	if bash.Artifact != "" {
		cmds = append(cmds, []string{"curl", "-o", bash.DownloadDir, bash.Artifact})
	}

	for _, cmd := range bash.Start {
		cmds = append(cmds, []string{"/bin/bash", "-c", cmd})
	}
	return cmds
}

func (bash BashExecutor) StopCmds(t Task) (cmds [][]string) {
	for _, cmd := range bash.Stop {
		cmds = append(cmds, []string{"/bin/bash", "-c", cmd})
	}
	return cmds
}

func (bash BashExecutor) GetEnv(t Task) []string {
	return bash.Env
}

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

func (docker DockerExecutor) StartCmds(t Task) (cmds [][]string) {
	docker.Name = t.Id()

	if t.TaskDef.ProvidePort {
		p := docker.ContainerPort
		if p == uint(0) {
			p = t.Port
		}
		docker.Ports = append(docker.Ports, fmt.Sprintf("%d:%d", t.Port, p))
	}

	// add the command
	cmds = append(cmds, []string{"docker", "pull", docker.Image})
	cmds = append(cmds, []string{"docker", "rm", "-f", docker.Name})

	main := []string{"docker", "run"}

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

	cmds = append(cmds, main)
	return cmds
}

func (docker DockerExecutor) StopCmds(t Task) [][]string {
	return [][]string{[]string{"docker", "stop", t.Id()}}
}

func (docker DockerExecutor) GetEnv(t Task) []string {
	return docker.Env
}
