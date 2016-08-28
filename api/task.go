package api

import "fmt"

func NewTask(cluster *Cluster, taskDef *TaskDefinition, service *Service, instance int) *Task {
	return &Task{
		Cluster:        cluster,
		TaskDefinition: taskDef,
		Service:        service.Name,
		Instance:       instance,
		Port:           taskDef.Port,
	}
}

// a depiction of a running task definition
type Task struct {
	Cluster        *Cluster
	TaskDefinition *TaskDefinition
	Service        string
	Instance       int
	Port           uint
	Host           string
	ProvidePort    bool

	api       SchedulerApi
	healthy   *bool
	scheduled *bool
}

func (task *Task) Scheduled() (bool, error) {
	if task.scheduled != nil {
		return *task.scheduled
	}

	ok, err := task.api.TaskScheduled(task.Id())
	if err != nil {
		return false, err
	}
	task.scheduled = ok
	return ok, nil
}

func (task *Task) Healthy() (bool, error) {
	if task.healthy != nil {
		return *task.healthy
	}

	ok, err := task.api.TaskHealthy(task.Id())
	if err != nil {
		return false, err
	}
	task.healthy = ok
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
	return fmt.Sprintf("%s-%s", task.Cluster.Name, task.Service)
}
