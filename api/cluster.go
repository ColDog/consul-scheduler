package api

type Cluster struct {
	Name      string
	Scheduler string
	Services  []string
}

func (c *Cluster) Validate(api *SchedulerApi) (errors []string) {
	if c.Name == "" {
		errors = append(errors, "cluster name is blank")
	}
	return errors
}

func (c *Cluster) Key() string {
	return "config/cluster/" + c.Name
}
