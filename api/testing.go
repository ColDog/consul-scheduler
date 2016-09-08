package api

func SampleContainer() *Container {
	return &Container{
		Name:     "test",
		Type:     "bash",
		Memory:   100000,
		CpuUnits: 100000,
		DiskUse:  100000,
		Executor: []byte(`{"start": "echo start", "stop": "echo stop"}`),
	}
}

func SampleTaskDefinition() *TaskDefinition {
	return &TaskDefinition{
		Name:        "test",
		Version:     1,
		ProvidePort: true,
		Tags:        []string{"test"},
		Containers: []*Container{
			SampleContainer(),
		},
	}
}

func SampleService() *Service {
	return &Service{
		Name:        "test",
		TaskName:    "test",
		TaskVersion: 0,
		Desired:     5,
		Max:         5,
	}
}

func SampleHost() *Host {
	return &Host{
		Name: "testinghost",
		CalculatedResources: &Resources{
			CpuUnits:  1000000000,
			Memory:    1000000000,
			DiskSpace: 1000000000,
		},
		ObservedResources: &Resources{
			CpuUnits:  1000000000,
			Memory:    1000000000,
			DiskSpace: 1000000000,
		},
		BaseResources: &Resources{
			CpuUnits:  1000000000,
			Memory:    1000000000,
			DiskSpace: 1000000000,
		},
		ReservedPorts: []uint{1000, 1001, 1002, 1003, 1004, 1005, 1006},
	}
}

func SampleTask() *Task {
	return NewTask(SampleCluster(), SampleTaskDefinition(), SampleService(), 1)
}

func SampleCluster() *Cluster {
	return &Cluster{
		Name:     "test",
		Services: []string{"test"},
	}
}
