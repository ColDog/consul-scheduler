package api

type TaskState string

const (
	STARTING TaskState = "starting"
	STOPPING TaskState = "stopping"
	RUNNING  TaskState = "running"
	FAILING  TaskState = "failing"
	EXITED   TaskState = "exited"
)

func (t TaskState) Healthy() bool {
	return t != FAILING && t != EXITED
}
