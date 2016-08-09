package main

import (
	"github.com/urfave/cli"
	. "github.com/coldog/scheduler/api"
	. "github.com/coldog/scheduler/agent"
	. "github.com/coldog/scheduler/scheduler"

	"os"
	"fmt"
	"io/ioutil"
	"github.com/ghodss/yaml"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

var ValidationErr error = errors.New("Validation Failed")

func valid(errs []string) error {
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Printf("err: %s\n", err)
		}
		return ValidationErr
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

func printOk() {
	fmt.Println("OK")
}

func main() {
	api := NewSchedulerApi()
	app := cli.NewApp()

	app.Name = "consul-scheduler"
	app.Version = "0.1.0"
	app.Author = "Colin Walker"
	app.Usage = "schedule tasks across a consul cluster."

	app.Commands = append(app.Commands, cli.Command{
		Name: "schedule",
		Usage: "trigger the scheduler",
		Action: func(c *cli.Context) error {
			api.TriggerScheduler()
			printOk()
			return nil
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "apply",
		Usage: "apply a configuration file to the cluster",
		Action: func(c *cli.Context) error {
			obj := struct {
				Clusters []Cluster `json:"clusters"`
				Services []Service `json:"services"`
				Tasks []TaskDefinition `json:"tasks"`
			}{}

			err := readYml(c.Args().First(), &obj)
			if err != nil {
				return err
			}

			for _, task := range obj.Tasks {
				err = valid(task.Validate(api))
				if err == nil {
					api.PutTaskDefinition(task)
					fmt.Printf("task: OK %s\n", task.Name)
				} else {
					fmt.Printf("task: FAIL %s\n", task.Name)
				}
			}

			for _, service := range obj.Services {
				err = valid(service.Validate(api))
				if err == nil {
					api.PutService(service)
					fmt.Printf("service: OK %s\n", service.Name)
				} else {
					fmt.Printf("service: FAIL %s\n", service.Name)
				}
			}

			for _, cluster := range obj.Clusters {
				err = valid(cluster.Validate(api))
				if err == nil {
					api.PutCluster(cluster)
					fmt.Printf("cluster: OK %s\n", cluster.Name)
				} else {
					fmt.Printf("cluster: FAIL %s\n", cluster.Name)
				}
			}

			return nil
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "combined",
		Usage: "start the scheduler and agent services together (recommended for quickstart)",
		Action: func(c *cli.Context) error {
			fmt.Println("==> starting...")

			m := NewMaster(api)
			ag := NewAgent(api)

			go m.Run()
			ag.Run()
			return nil
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "scheduler",
		Usage: "start the scheduler service",
		Action: func(c *cli.Context) error {
			fmt.Println("==> starting...")

			m := NewMaster(api)
			m.Run()
			return nil
		},
	})

	app.Commands = append(app.Commands, cli.Command{
		Name: "agent",
		Usage: "start the agent service",
		Action: func(c *cli.Context) error {
			fmt.Println("==> starting...")

			ag := NewAgent(api)
			ag.Run()
			return nil
		},
	})

	app.Run(os.Args)
}
