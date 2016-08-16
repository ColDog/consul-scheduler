package executors

import (
	. "github.com/coldog/scheduler/api"
	"encoding/json"
	"fmt"
)

func GetExecutor(c Container) Executor {

	if c.Type == "docker" {
		res := DockerExecutor{}
		err := json.Unmarshal(c.Executor, &res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}
		return res
	} else if c.Type == "bash" {
		res := BashExecutor{}
		err := json.Unmarshal(c.Executor, &res)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return nil
		}
		return res
	}

	return nil
}
