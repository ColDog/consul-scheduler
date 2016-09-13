package api

type TaskState string

const (
	PENDING  TaskState = ""
	STARTING TaskState = "starting"
	STARTED  TaskState = "started"
	RUNNING  TaskState = "running"
	WARNING  TaskState = "warning"
	FAILING  TaskState = "failing"
	STOPPING TaskState = "stopping"
	STOPPED  TaskState = "stopped"
	EXITED   TaskState = "exited"
)

func (t TaskState) Healthy() bool {
	return t == RUNNING || t == STARTED
}
