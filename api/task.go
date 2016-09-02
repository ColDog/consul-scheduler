package api

import "fmt"

func NewTask(cluster *Cluster, taskDef *TaskDefinition, service *Service, instance int) *Task {
	t := &Task{
		Cluster:        cluster.Name,
		TaskDefinition: taskDef,
		Service:        service.Name,
		Instance:       instance,
		Port:           taskDef.Port,
	}

	for _, cont := range t.TaskDefinition.Containers {
		for _, chk := range cont.Checks {
			chk.TaskID = t.Id()
		}
	}

	return t
}

// a depiction of a running task definition
type Task struct {
	Cluster        string          `json:"cluster"`
	TaskDefinition *TaskDefinition `json:"task_definition"`
	Service        string          `json:"service"`
	Instance       int             `json:"instance"`
	Port           uint            `json:"port"`
	Host           string          `json:"host"`
	Scheduled      bool            `json:"scheduled"`
	Rejected       bool            `json:"rejected"`
	RejectReason   string          `json:"reject_reason"`
	api            SchedulerApi
	healthy        *bool
}

func (task *Task) AllPorts() []uint {
	if task.Port != 0 {
		return append(task.TaskDefinition.AllPorts(), task.Port)
	} else {
		return task.TaskDefinition.AllPorts()
	}
}

func (task *Task) HasChecks() bool {
	for _, c := range task.TaskDefinition.Containers {
		if len(c.Checks) > 0 {
			return true
		}
	}
	return false
}

func (task *Task) Healthy() (bool, error) {
	if task.healthy != nil {
		return *task.healthy, nil
	}

	ok, err := task.api.TaskHealthy(task)
	if err != nil {
		return false, err
	}
	task.healthy = &ok
	return ok, nil
}

func (task *Task) Validate(api SchedulerApi) (errors []string) {
	if task.Service == "" {
		errors = append(errors, "service name is blank")
	}

	_, err := api.GetService(task.Service)
	if err != nil {
		errors = append(errors, "service does not exist")
	}

	return errors
}

func MakeTaskId(c *Cluster, s *Service, i int) string {
	return fmt.Sprintf("%s-%s-%v-%v", c.Name, s.Name, s.TaskVersion, i)
}

// consul name:  <cluster_name>.<service_name>
// consul id:    <name>.<task_version>-<instance>
func (task *Task) Id() string {
	return fmt.Sprintf("%s-%v-%v", task.Name(), task.TaskDefinition.Version, task.Instance)
}

func (task *Task) Name() string {
	return fmt.Sprintf("%s-%s", task.Cluster, task.Service)
}
