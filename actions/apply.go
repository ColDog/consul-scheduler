package actions

import (
	"github.com/coldog/sked/api"
	"github.com/ghodss/yaml"

	"fmt"
	"errors"
	"io/ioutil"
)

var ErrValidationFailure error = errors.New("Validation Failed")

func valid(errs []string) error {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Printf("err: %s\n", err)
		}
		return ErrValidationFailure
	}
	return nil
}

func readYml(file string, res interface{}) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal([]byte(data), res)
	return err
}

func ApplyConfig(arg string, a api.SchedulerApi) error {
	obj := struct {
		Clusters []*api.Cluster `json:"clusters"`
		Services []*api.Service `json:"services"`
		Tasks []*api.TaskDefinition `json:"tasks"`
	}{}

	err := readYml(arg, &obj)
	if err != nil {
		return err
	}

	for _, task := range obj.Tasks {
		err = valid(task.Validate(a))
		if err == nil {
			a.PutTaskDefinition(task)
			fmt.Printf("task: OK %s\n", task.Name)
		} else {
			fmt.Printf("task: FAIL %s\n", task.Name)
		}
	}

	for _, service := range obj.Services {
		err = valid(service.Validate(a))
		if err == nil {
			a.PutService(service)
			fmt.Printf("service: OK %s\n", service.Name)
		} else {
			fmt.Printf("service: FAIL %s\n", service.Name)
		}
	}

	for _, cluster := range obj.Clusters {
		err = valid(cluster.Validate(a))
		if err == nil {
			a.PutCluster(cluster)
			fmt.Printf("cluster: OK %s\n", cluster.Name)
		} else {
			fmt.Printf("cluster: FAIL %s\n", cluster.Name)
		}
	}

	return nil
}
