package api

import "fmt"

func NewTask(cluster *Cluster, taskDef *TaskDefinition, service *Deployment, instance int) *Task {
	t := &Task{
		Cluster:        cluster.Name,
		TaskDefinition: taskDef,
		Deployment:     service.Name,
		Instance:       instance,
		ProvidedPorts:  map[string]uint{},
	}

	return t
}

// a depiction of a running task definition
type Task struct {
	Cluster        string          `json:"cluster"`
	TaskDefinition *TaskDefinition `json:"task_definition"`
	Deployment     string          `json:"service"`
	Instance       int             `json:"instance"`
	Host           string          `json:"host"`
	Scheduled      bool            `json:"scheduled"`
	Rejected       bool            `json:"rejected"`
	RejectReason   string          `json:"reject_reason"`
	ProvidedPorts  map[string]uint `json:"provided_ports"`
}

func (task *Task) HasChecks() bool {
	for _, c := range task.TaskDefinition.Containers {
		if len(c.Checks) > 0 {
			return true
		}
	}
	return false
}

func (task *Task) Validate(api SchedulerApi) (errors []string) {
	if task.Deployment == "" {
		errors = append(errors, "deployment name is blank")
	}

	_, err := api.GetDeployment(task.Deployment)
	if err != nil {
		errors = append(errors, "deployment does not exist")
	}

	return errors
}

func MakeTaskId(c *Cluster, s *Deployment, i int) string {
	return fmt.Sprintf("%s-%s-%v-%v", c.Name, s.Name, s.TaskVersion, i)
}

// consul name:  <cluster_name>.<service_name>
// consul id:    <name>.<task_version>-<instance>
func (task *Task) Id() string {
	return task.ID()
}

// refactoring to more golang naming practices, eventually only use this method below.
func (task *Task) ID() string {
	return fmt.Sprintf("%s-%v-%v", task.Name(), task.TaskDefinition.Version, task.Instance)
}

func (task *Task) Name() string {
	return fmt.Sprintf("%s-%s", task.Cluster, task.Deployment)
}
