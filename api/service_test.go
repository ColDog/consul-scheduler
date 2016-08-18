package api

func sampleService() *Service {
	return &Service{
		Name: "test",
		TaskName: "test",
		TaskVersion: 0,
		Desired: 1,
		Max: 1,
	}
}
