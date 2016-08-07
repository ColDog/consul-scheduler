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

func RegisterTaskDefinitionCommands(api *SchedulerApi, app *cli.App)  {
	app.Commands = append(app.Commands, cli.Command{
		Name: "task-definition",
		Usage: "create and update task definitions",
		Subcommands: []cli.Command{
			{
				Name: "apply",
				ArgsUsage: "file",
				Action: func(c *cli.Context)  error {
					t := TaskDefinition{}
					err := readYml(c.Args().First(), &t)
					if err != nil {
						return err
					}

					err = valid(t.Validate(api))
					if err == nil {
						api.PutTaskDefinition(t)
					}

					return err
				},
			},
			{
				Name: "show",
				ArgsUsage: "name",
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
					}

					return err
				},
			},
			{
				Name: "show",
				ArgsUsage: "name",
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
				Name: "remove",
				ArgsUsage: "name",
				Action: func(c *cli.Context) error {
					name := c.Args().First()
					if name == "" {
						return errors.New("err: name not provided")
					}

					api.DelService(name)
					return nil
				},
			},
		},
	})
}

func RegisterClusterCommands(api *SchedulerApi, app *cli.App) {
	app.Commands = append(app.Commands, cli.Command{
		Name: "clusters",
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
