package api

type Host struct {
	Name          string            `json:"name"`
	Memory        uint64            `json:"memory"`
	Draining      bool              `json:"draining"`
	DiskSpace     uint64            `json:"disk_space"`
	CpuUnits      uint64            `json:"cpu_units"`
	ReservedPorts []uint            `json:"reserved_ports"`
	HealthCheck   string            `json:"health_check"`
	Tags          map[string]string `json:"tags"`
}
