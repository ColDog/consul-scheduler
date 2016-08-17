package executors

import (
	. "github.com/coldog/scheduler/api"
	"testing"
	"bytes"
)

func TestBash(t *testing.T) {
	b := BashExecutor{
		Start: []string{"echo", "hello"},
		Stop: []string{"echo", "stop"},
	}

	t := Task{

	}

	b.StartTask()

}
