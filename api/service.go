package api

import "fmt"

type Service struct {
	Name       string `json:"name"`
	Cluster    string `json:"cluster"`
	Deployment string `json:"deployment"`
	Container  string `json:"container"`
	PortName   string `json:"port_name"`
	Protocol   string `json:"protocol"`
}

func (s *Service) Validate(api SchedulerApi) (errors []string) {
	if s.Name == "" {
		s.Name = s.Cluster + "-" + s.Deployment + "-" + s.Container
	}

	_, err := api.GetCluster(s.Cluster)
	if err != nil {
		errors = append(errors, "cluster: " + err.Error())
	}

	_, err = api.GetDeployment(s.Deployment)
	if err != nil {
		errors = append(errors, "deployment: " + err.Error())
	}

	return errors
}

type Endpoint struct {
	Service  string `json:"service"`
	Host     string `json:"address"`
	Port     uint   `json:"port"`
	Protocol string `json:"protocol"`
}

func (e *Endpoint) Address() string {
	return fmt.Sprintf("%s://%s:%s", e.Protocol, e.Host, e.Port)
}
