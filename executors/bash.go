package executors

import (
	. "github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/tools"

	"fmt"
)

// a type of executor
type BashExecutor struct {
	Start       []string `json:"start"`
	Stop        []string `json:"stop"`
	Env         []string `json:"env"`
	Artifact    string   `json:"artifact"`
	DownloadDir string   `json:"download_dir"`
}

func (bash BashExecutor) StartTask(t Task) error {
	if t.TaskDef.ProvidePort {
		bash.Env = append(bash.Env, fmt.Sprintf("SCHEDULED_PORT=%d", t.Port))
	}

	if bash.Artifact != "" {
		err := tools.Exec(bash.Env, "curl", "-o", bash.DownloadDir, bash.Artifact)
		if err != nil {
			return err
		}
	}

	for _, cmd := range bash.Start {
		err := tools.Exec(bash.Env, "/bin/bash", "-c", cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bash BashExecutor) StopTask(t Task) (err error) {
	for _, cmd := range bash.Start {
		err = tools.Exec(bash.Env, "/bin/bash", "-c", cmd)
	}
	return err
}
