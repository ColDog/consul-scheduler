package api

type Service struct {
	Name       string `json:"name"`
	Cluster    string `json:"cluster"`
	Deployment string `json:"deployment"`
	Container  string `json:"container"`
	PortName   string `json:"port_name"`
}

type Endpoint struct {
	Service  string `json:"service"`
	Host     string `json:"address"`
	Port     uint   `json:"port"`
	Protocol string `json:"protocol"`
}
