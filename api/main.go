package api

import (
	"encoding/json"
	"fmt"
)

// the executor is a stop and startable executable that can be passed to an agent to run.
// all commands should have at least one string in the array or else a panic will be thrown
// by the agent.
type Executor interface {
	StartTask(t Task) error
	StopTask(t Task) error
}

type Validatable interface {
	Validate(*SchedulerApi) []string
}

// encode and decode functions, the encode function will panic if the json marshalling fails.
func encode(item interface{}) []byte {
	res, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	return res
}

func decode(data []byte, item interface{}) {
	json.Unmarshal(data, item)
}
