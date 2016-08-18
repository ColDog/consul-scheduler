package api

func sampleCluster() *Cluster {
	return &Cluster{
		Name: "test",
		Scheduler: "default",
		Services: []string{"test"},
	}
}
