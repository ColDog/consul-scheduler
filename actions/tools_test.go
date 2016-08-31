package actions

import (
	"github.com/coldog/sked/api"

	"testing"
	"math/rand"
	"fmt"
)

func TestApplyPrint(t *testing.T) {
	a := api.NewMockApi()

	for i := 0; i < 10; i++ {
		t := api.SampleTask()
		t.Scheduled = true
		t.Instance = i
		t.Host = fmt.Sprintf("local-%d", i)
		a.PutTask(t)
	}

	ListTasks(a, "", "", "")
}

func TestTableRows(t *testing.T) {
	fmt.Println("\n\n")
	for i := 0; i < 10; i++ {
		x := ""
		for i := 0; i < rand.Intn(20); i++ {
			x += "a"
		}

		tableRow(x, "longsdfgsdf", "sfdgsdf", "sdfss", "adsfdfgsdfgsdfgsdfgsdfgsdfgsfg")
	}
}
