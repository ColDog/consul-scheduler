package api

// validations:
// 	- task must exist
//	- min must be less than max
//	- max must be greater than or equal to desired
//	- min must be less than or equal to desired
//
type Service struct {
	Name        string `json:"name"`
	Scheduler   string `json:"scheduler"`
	TaskName    string `json:"task_name"`
	TaskVersion uint   `json:"task_version"`
	Desired     int    `json:"desired"`
	Min         int    `json:"min"`
	Max         int    `json:"max"`
}

func (service *Service) Validate(api SchedulerApi) (errors []string) {
	_, err := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		errors = append(errors, "task ("+service.TaskName+") does not exist")
	}

	if service.Name == "" {
		errors = append(errors, "service name is blank")
	}

	if service.Min > service.Max {
		errors = append(errors, "min is greater than max")
	}

	if service.Min > service.Desired {
		errors = append(errors, "min is greater than desired")
	}

	if service.Max < service.Desired {
		errors = append(errors, "max is less than desired")
	}

	return errors
}

func (s *Service) Key() string {
	return "config/service/" + s.Name
}
