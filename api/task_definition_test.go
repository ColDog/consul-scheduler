package api

import (
	"testing"
	"fmt"
	"github.com/coldog/sked/tools"
)

func TestTaskDefinition_Counts(t *testing.T) {
	td := SampleTaskDefinition()

	fmt.Printf("%+v\n", td.Counts())
	tools.Assert(t, td.Counts().Memory == 100000, "mem does not match")
}
