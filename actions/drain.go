package actions

import "github.com/coldog/sked/api"

func Drain(hostName string, a api.SchedulerApi) error {
	host, err := a.GetHost(hostName)
	if err != nil {
		return err
	}

	host.Draining = true
	return a.PutHost(host)
}
