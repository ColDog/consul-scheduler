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

func (bash BashExecutor) commands() (cmds [][]string) {
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
	Image 		string
	Name 		string
	Cmd 		string
	Entry 		string
	Ports 		[]string
	Env 		[]string
	WorkDir 	string
	Volumes 	[]string
	Flags 		[]string
}

func (docker DockerExecutor) commands() (cmds [][]string) {

	// add the command
	cmds = append(cmds, []string{"docker", "pull", docker.Image})

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

	for _, f := range docker.Flags {
		main = append(main, f)
	}

	for _, v := range docker.Volumes {
		main = append(main, "-v", v)
	}

	for _, e := range docker.Env {
		main = append(main, "-e", e)
	}

	for _, p := range docker.Ports {
		main = append(main, "-p", p)
	}

	main = append(main, docker.Image)

	cmds = append(cmds, main)
	return cmds
}
