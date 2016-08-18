package api

//Start       []string `json:"start"`
//Stop        []string `json:"stop"`
//Env         []string `json:"env"`
//Artifact    string   `json:"artifact"`
//DownloadDir string   `json:"download_dir"`
func sampleContainer() *Container {
	return &Container{
		Name: "test",
		Type: "bash",
		Executor: []byte(`{"start": ["echo start"], "stop": ["echo stop"]}`),
	}
}

func sampleTaskDefinition() *TaskDefinition {
	return &TaskDefinition{
		Name: "test",
		Version: 0,
		ProvidePort: true,
		Tags: []string{"test"},
		Containers: []*Container{
			sampleContainer(),
		},
	}
}
