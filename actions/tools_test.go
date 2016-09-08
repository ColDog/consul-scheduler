package actions

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/backends/mock"

	"fmt"
	"testing"
)

func TestApplyPrint(t *testing.T) {
	a := mock.NewMockApi()

	for i := 0; i < 10; i++ {
		t := api.SampleTask()
		t.Scheduled = true
		t.Instance = i
		t.Host = fmt.Sprintf("local-%d", i)
		a.PutTask(t)
	}

	ListTasks(a, "", "", "")
}
