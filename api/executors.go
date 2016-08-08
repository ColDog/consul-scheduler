package api

// a type of executor
type BashExecutor struct {
	Cmd 		string
	Args 		[]string
	Env 		[]string
	Kill 		string
	Artifact 	string
	DownloadDir 	string
}

func (bash BashExecutor) Commands() (cmds [][]string) {
	if bash.Artifact != "" {
		cmds = append(cmds, []string{"curl", "-o", bash.DownloadDir, bash.Artifact})
	}

	main := make([]string, 0)
	main = append(main, bash.Cmd)
	main = append(main, bash.Args...)
	cmds = append(cmds, main)
	return cmds
}

// a type of executor
type DockerExecutor struct {
	Image 		string		`json:"image"`
	Name 		string		`json:"name"`
	Cmd 		string		`json:"cmd"`
	Entry 		string		`json:"entry"`
	ContainerPort 	uint		`json:"container_port"`
	Ports 		[]string	`json:"ports"`
	Env 		[]string	`json:"env"`
	WorkDir 	string		`json:"work_dir"`
	Volumes 	[]string	`json:"volumes"`
	Flags 		[]string	`json:"flags"`
}

func (docker DockerExecutor) Commands() (cmds [][]string) {

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
