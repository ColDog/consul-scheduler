package cli

import (
	"github.com/urfave/cli"
	. "github.com/coldog/scheduler/api"
	"github.com/ghodss/yaml"

	"errors"
	"fmt"
	"io/ioutil"
)

var ValidationErr error = errors.New("err: validation error")
var NotFoundErr error = errors.New("err: not found")
var NotImplementedErr error = errors.New("err: not implemented")

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

func printYaml(obj interface{}) {
	data, _ := yaml.Marshal(obj)
	fmt.Printf("\n%s\n", data)
}

func printOk() {
	fmt.Println("OK")
}



func RegisterConfigCommands(api *SchedulerApi, app *cli.App)  {
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
}

func RegisterTaskDefinitionCommands(api *SchedulerApi, app *cli.App)  {
	app.Commands = append(app.Commands, cli.Command{
		Name: "task-definition",
		Usage: "create, update and view task definitions",
		Subcommands: []cli.Command{
			{
				Name: "apply",
				ArgsUsage: "file",
				Usage: "add or update a task definition",
				Action: func(c *cli.Context)  error {
					t := TaskDefinition{}
					err := readYml(c.Args().First(), &t)
					if err != nil {
						return err
					}

					err = valid(t.Validate(api))
					if err == nil {
						api.PutTaskDefinition(t)
						printOk()
					}

					return err
				},
			},
			{
				Name: "list",
				Usage: "list names of currently available task definitions",
				Action: func(c *cli.Context)  error {
					list := api.ListTaskDefinitionNames()
					printYaml(list)
					return nil
				},
			},
			{
				Name: "show",
				ArgsUsage: "name",
				Usage: "show a task definition by version and name",
				Flags: []cli.Flag{
					cli.IntFlag{Name: "version, v", Value: 1},
				},
				Action: func(c *cli.Context)  error {
					t, ok := api.GetTaskDefinition(c.Args().First(), c.Uint("version"))

					if !ok {
						return NotFoundErr
					}

					printYaml(t)
					return nil
				},
			},
		},
	})
}

func RegisterTaskCommands(api *SchedulerApi, app *cli.App)  {
	app.Commands = append(app.Commands, cli.Command{
		Name: "tasks",
		Usage: "create, update and view tasks",
		Subcommands: []cli.Command{
			{
				Name: "create",
				Usage: "create a new task",
				Action: func(c *cli.Context) error {
					t := Task{}
					err := readYml(c.Args().First(), &t)
					if err != nil {
						return err
					}

					err = valid(t.Validate(api))
					if err == nil {
						api.PutTask(t)
					}

					return err
				},
			},
			{
				Name: "list",
				Usage: "list running tasks",
				Action: func(c *cli.Context) error {
					tasks := api.RunningTasksOnHost(c.Args().First())

					for _, t := range tasks {
						fmt.Printf("%s => ok: %v, host: %s\n", t.ServiceID, t.Passing, t.Task.Host)
					}

					return nil
				},
			},
			{
				Name: "remove",
				Action: func(c *cli.Context) error {
					t := Task{}
					err := readYml(c.Args().First(), &t)
					if err != nil {
						return err
					}

					api.DelTask(t)
					return nil
				},
			},
		},
	})
}

func RegisterServiceCommands(api *SchedulerApi, app *cli.App) {
	app.Commands = append(app.Commands, cli.Command{
		Name: "services",
		Usage: "create, update and view services",
		Subcommands: []cli.Command{
			{
				Name: "apply",
				ArgsUsage: "file",
				Usage: "create a new service",
				Action: func(c *cli.Context) error {
					t := Service{}
					err := readYml(c.Args().First(), &t)
					if err != nil {
						return err
					}

					err = valid(t.Validate(api))
					if err == nil {
						api.PutService(t)
						printOk()
					}

					return err
				},
			},
			{
				Name: "show",
				ArgsUsage: "name",
				Usage: "describe a service",
				Action: func(c *cli.Context) error {
					name := c.Args().First()
					if name == "" {
						return errors.New("err: name not provided")
					}

					ser, ok := api.GetService(name)
					if !ok {
						return NotFoundErr
					}

					printYaml(ser)
					return nil
				},
			},
			{
				Name: "list",
				Usage: "list all services currently registered",
				Action: func(c *cli.Context) error {
					services := api.ListServiceNames()
					printYaml(services)
					return nil
				},
			},
			{
				Name: "remove",
				ArgsUsage: "name",
				Usage: "remove a specific service",
				Action: func(c *cli.Context) error {
					name := c.Args().First()
					if name == "" {
						return errors.New("err: name not provided")
					}

					api.DelService(name)
					printOk()
					return nil
				},
			},
		},
	})
}

func RegisterClusterCommands(api *SchedulerApi, app *cli.App) {
	app.Commands = append(app.Commands, cli.Command{
		Name: "clusters",
		Usage: "create, update and view clusters",
		Subcommands: []cli.Command{
			{
				Name: "create",
				ArgsUsage: "name",
				Action: func(c *cli.Context) error {
					return NotImplementedErr
				},
			},
			{
				Name: "show",
				ArgsUsage: "name",
				Action: func(c *cli.Context) error {
					return NotImplementedErr
				},
			},
			{
				Name: "list",
				Action: func(c *cli.Context) error {
					list := api.ListClusters()
					printYaml(list)
					return nil
				},
			},
			{
				Name: "remove",
				Action: func(c *cli.Context) error {
					return NotImplementedErr
				},
			},
		},
	})
}
