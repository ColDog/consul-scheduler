package actions

import (
	"github.com/coldog/sked/api"
	"github.com/ghodss/yaml"

	"errors"
	"fmt"
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
		Clusters        []*api.Cluster        `json:"clusters"`
		Deployments     []*api.Deployment     `json:"deployments"`
		TaskDefinitions []*api.TaskDefinition `json:"tasks"`
		Services        []*api.Service        `json:"services"`
		Deployment      *api.Deployment       `json:"deployment"`
		TaskDefinition  *api.TaskDefinition   `json:"task_definition"`
		Cluster         *api.Cluster          `json:"cluster"`
		Service         *api.Service          `json:"service"`
	}{}

	err := readYml(arg, &obj)
	if err != nil {
		return err
	}

	for _, task := range obj.TaskDefinitions {
		err = valid(task.Validate(a))
		if err == nil {
			a.PutTaskDefinition(task)
			fmt.Printf("task_definition: OK %s\n", task.Name)
		} else {
			fmt.Printf("task_definition: FAIL %s\n", task.Name)
		}
	}

	for _, deploy := range obj.Deployments {
		err = valid(deploy.Validate(a))
		if err == nil {
			a.PutDeployment(deploy)
			fmt.Printf("deployment: OK %s\n", deploy.Name)
		} else {
			fmt.Printf("deployment: FAIL %s\n", deploy.Name)
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

	for _, service := range obj.Services {
		err = valid(service.Validate(a))
		if err == nil {
			a.PutService(service)
			fmt.Printf("service: OK %s\n", service.Name)
		} else {
			fmt.Printf("service: FAIL %s\n", service.Name)
		}
	}

	if obj.Service != nil {
		err = valid(obj.Service.Validate(a))
		if err == nil {
			a.PutService(obj.Service)
			fmt.Printf("service: OK %s\n", obj.Service.Name)
		} else {
			fmt.Printf("service: FAIL %s\n", obj.Service.Name)
		}
	}

	if obj.Deployment != nil {
		err = valid(obj.Deployment.Validate(a))
		if err == nil {
			a.PutDeployment(obj.Deployment)
			fmt.Printf("deployment: OK %s\n", obj.Deployment.Name)
		} else {
			fmt.Printf("deployment: FAIL %s\n", obj.Deployment.Name)
		}
	}

	if obj.Cluster != nil {
		err = valid(obj.Cluster.Validate(a))
		if err == nil {
			a.PutCluster(obj.Cluster)
			fmt.Printf("deployment: OK %s\n", obj.Cluster.Name)
		} else {
			fmt.Printf("deployment: FAIL %s\n", obj.Cluster.Name)
		}
	}

	if obj.TaskDefinition != nil {
		err = valid(obj.Cluster.Validate(a))
		if err == nil {
			a.PutTaskDefinition(obj.TaskDefinition)
			fmt.Printf("task_definition: OK %s\n", obj.TaskDefinition.Name)
		} else {
			fmt.Printf("task_definition: FAIL %s\n", obj.TaskDefinition.Name)
		}
	}

	return nil
}
