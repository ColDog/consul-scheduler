package api

type TaskState string

const (
	STARTING TaskState = "starting"
	STOPPING TaskState = "stopping"
	STOPPED  TaskState = "stopped"
	RUNNING  TaskState = "running"
	WARNING  TaskState = "warning"
	FAILING  TaskState = "failing"
	EXITED   TaskState = "exited"
)

func (t TaskState) Healthy() bool {
	return t == RUNNING
}
