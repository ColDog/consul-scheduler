package api

type Host struct {
	Name                string            `json:"name"`
	Draining            bool              `json:"draining"`
	ReservedPorts       []uint            `json:"reserved_ports"`
	HealthCheck         string            `json:"health_check"`
	ObservedResources   *Resources        `json:"observed_resources"`
	CalculatedResources *Resources        `json:"calculated_resources"`
	BaseResources       *Resources        `json:"base_resources"`
	Cluster             string            `json:"cluster"`
	Tags                map[string]string `json:"tags"`
}

type Resources struct {
	Memory        int64   `json:"memory"`
	CpuUnits      int64   `json:"cpu_units"`
	DiskSpace     int64   `json:"disk_space"`
	MemUsePercent float64 `json:"mem_use_percent"`
}
