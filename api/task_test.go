package api

func sampleTask() *Task {
	return NewTask(sampleCluster(), sampleTaskDefinition(), sampleService(), 1)
}
