package api

type Host struct {
	Name          string            `json:"name"`
	Memory        uint64            `json:"memory"`
	Draining      bool              `json:"draining"`
	DiskSpace     uint64            `json:"disk_space"`
	CpuUnits      uint64            `json:"cpu_units"`
	MemUsePercent float64           `json:"mem_use_percent"`
	ReservedPorts []uint            `json:"reserved_ports"`
	Tags          map[string]string `json:"tags"`
}
