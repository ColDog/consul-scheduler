package api

type Cluster struct {
	Name       string   `json:"name"`
	Datacenter string   `json:"datacenter"`
	Services   []string `json:"services"`
	Hosts      []string `json:"hosts"`
}

func (c *Cluster) Validate(api SchedulerApi) (errors []string) {
	if c.Name == "" {
		errors = append(errors, "cluster name is blank")
	}
	return errors
}

func (c *Cluster) Key() string {
	return "config/cluster/" + c.Name
}
