package main

import (
	"encoding/json"
	"fmt"
)

type Validatable interface {
	Validate(*SchedulerApi) []string
}

// a check passed along to consul
type Check struct  {
	Id 		string			`json:"id"`
	Name 		string			`json:"name"`
	Http 		string			`json:"http"`
	Tcp 		string			`json:"tcp"`
	Script 		string			`json:"script"`
	Interval 	string			`json:"interval"`
	Timeout 	string			`json:"timeout"`
	Ttl 		string			`json:"ttl"`
}

// a runnable task description
// validations:
// 	- version must be new
//
type TaskDefinition struct {
	Name 		string			`json:"name"`
	Version 	uint			`json:"version"`
	ProvidePort 	bool			`json:"provide_port"`
	Port 		uint			`json:"port"`
	Memory 		uint64			`json:"memory"`
	CpuUnits 	uint64			`json:"cpu_units"`
	Containers 	[]Container		`json:"containers"`
	Checks 		[]Check                 `json:"checks"`
}

func (task TaskDefinition) Validate(api *SchedulerApi) (errors []string) {
	_, ok := api.GetTaskDefinition(task.Name, task.Version)
	if ok {
		errors = append(errors, "version already provisioned")
	}
	return errors
}

type Container struct {
	Name 		string
	Executor 	string			`json:"executor"`
	Bash		BashExecutor		`json:"bash"`
	Docker		DockerExecutor		`json:"docker"`
}

func (t TaskDefinition) Key() string {
	return "config/task/" + t.Name
}

// validations:
// 	- task must exist
//	- min must be less than max
//	- max must be greater than or equal to desired
//	- min must be less than or equal to desired
//
type Service struct {
	Name 		string
	TaskName 	string
	TaskVersion 	uint
	Desired 	int
	Min 		int
	Max 		int
}

func (service Service) Validate(api *SchedulerApi) (errors []string) {
	_, ok := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if !ok {
		errors = append(errors, "task does not exist")
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

func (s Service) Key() string {
	return "config/service/" + s.Name
}

type Cluster struct {
	Name 		string
	Scheduler 	string
	Services 	[]string
}

func (cluster Cluster) Validate(api *SchedulerApi) (errors []string) {
	if cluster.Name == "" {
		errors = append(errors, "cluster name is blank")
	}
	return errors
}

func (c Cluster) Key() string {
	return "config/cluster/" + c.Name
}

func NewTask(cluster Cluster, taskDef TaskDefinition, service Service, instance int) Task {
	return Task{
		Cluster: cluster,
		TaskDef: taskDef,
		Service: service.Name,
		Instance: instance,
		Port: taskDef.Port,
	}
}

// a depiction of a running task definition
type Task struct {
	Cluster 	Cluster
	TaskDef 	TaskDefinition
	Service 	string
	LocalPid 	string
	Instance 	int
	Port 		uint
	Host 		string
}

func (task Task) Validate(api *SchedulerApi) (errors []string) {
	if task.Service == "" {
		errors = append(errors, "service name is blank")
	}

	_, ok := api.GetService(task.Service)
	if !ok {
		errors = append(errors, "service does not exist")
	}

	return errors
}

// consul name:  <cluster_name>.<service_name>
// consul id:    <name>.<task_version>-<instance>
func (task Task) Id() string {
	return fmt.Sprintf("%s_%v-%v", task.Name(), task.TaskDef.Version, task.Instance)
}

func (task Task) Name() string {
	return fmt.Sprintf("%s_%s", task.Cluster.Name, task.Service)
}

type Host struct {
	Name 		string
	Memory 		uint64
	DiskSpace 	uint64
	CpuUnits 	uint64
	ReservedPorts 	[]uint
	PortSelection 	[]uint
}

func (host Host) IsPortAvailable(port uint) bool {
	for _, resPort := range host.ReservedPorts {
		if port == resPort {
			return false
		}
	}
	return true
}

func (host Host) AvailablePort() uint {
	for i := 0; i < len(host.PortSelection); i++ {
		if host.IsPortAvailable(host.PortSelection[i]) {
			return host.PortSelection[i]
		}
	}
	return 0
}

func encode(item interface{}) []byte {
	res, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	return res
}

func decode(data []byte, item interface{}) {
	json.Unmarshal(data, item)
}
