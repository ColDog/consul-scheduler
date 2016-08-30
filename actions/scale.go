package actions

import "github.com/coldog/sked/api"

func Scale(serviceName string, desired int, a api.SchedulerApi) error {
	service, err := a.GetService(serviceName)
	if err != nil {
		return err
	}

	if desired < service.Min {
		service.Min = desired
	}

	if desired > service.Max {
		service.Max = desired
	}

	service.Desired = desired

	return a.PutService(service)
}
