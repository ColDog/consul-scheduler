package api

import (
	"encoding/json"
	"fmt"
	"github.com/coldog/scheduler/executors"
	"log"
)

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	Start(t Task) error
	Stop(t Task) error
}

type Validatable interface {
	Validate(*SchedulerApi) []string
}

// a check passed along to consul
type Check struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	Http            string `json:"http"`
	Tcp             string `json:"tcp"`
	Script          string `json:"script"`
	AddProvidedPort bool   `json:"add_provided_port"`
	Interval        string `json:"interval"`
	Timeout         string `json:"timeout"`
	Ttl             string `json:"ttl"`
}

// a runnable task description
// validations:
// 	- version must be new
//
type TaskDefinition struct {
	Name        string      `json:"name"`
	Version     uint        `json:"version"`
	ProvidePort bool        `json:"provide_port"`
	Port        uint        `json:"port"`
	Tags        []string    `json:"tags"`
	Memory      uint64      `json:"memory"`
	CpuUnits    uint64      `json:"cpu_units"`
	Containers  []Container `json:"containers"`
	Checks      []Check     `json:"checks"`
}

func (task TaskDefinition) Validate(api *SchedulerApi) (errors []string) {
	_, err := api.GetTaskDefinition(task.Name, task.Version)
	if err == nil {
		errors = append(errors, "version already provisioned")
	}
	return errors
}

type Container struct {
	Name     string	          `json:"name"`
	Type     string           `json:"type"`
	Executor json.RawMessage  `json:"executor"`
}

func (c Container) GetExecutor() (res Executor) {

	if c.Type == "docker" {
		res = executors.DockerExecutor{}
	} else if c.Type == "bash" {
		res = executors.BashExecutor{}
	}

	if res != nil {
		err := json.Unmarshal(c.Executor, &res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}
	}

	return res, nil
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
	Name        string `json:"name"`
	TaskName    string `json:"task_name"`
	TaskVersion uint   `json:"task_version"`
	Desired     int    `json:"desired"`
	Min         int    `json:"min"`
	Max         int    `json:"max"`
}

func (service Service) Validate(api *SchedulerApi) (errors []string) {
	_, err := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		errors = append(errors, "task ("+service.TaskName+") does not exist")
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
	Name      string
	Scheduler string
	Services  []string
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

// A running task compiled by the agents
type RunningTask struct {
	ServiceID string
	Task      Task
	Service   Service
	Passing   bool
	Exists    bool
}

func NewTask(cluster Cluster, taskDef TaskDefinition, service Service, instance int) Task {
	return Task{
		Cluster:  cluster,
		TaskDef:  taskDef,
		Service:  service.Name,
		Instance: instance,
		Port:     taskDef.Port,
	}
}

// a depiction of a running task definition
type Task struct {
	Cluster  Cluster
	TaskDef  TaskDefinition
	Service  string
	LocalPid string
	Instance int
	Port     uint
	Host     string
	Stopped  bool
}

func (task Task) Validate(api *SchedulerApi) (errors []string) {
	if task.Service == "" {
		errors = append(errors, "service name is blank")
	}

	_, err := api.GetService(task.Service)
	if err != nil {
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
	Name          string
	Memory        uint64
	DiskSpace     uint64
	CpuUnits      uint64
	ReservedPorts []uint
	PortSelection []uint
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
