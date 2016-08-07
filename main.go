package main

import (
	"github.com/urfave/cli"
	. "github.com/coldog/scheduler/api"
	. "github.com/coldog/scheduler/cli"
	. "github.com/coldog/scheduler/agent"
	. "github.com/coldog/scheduler/scheduler"

	"os"
	"fmt"
)

func main() {
	api := NewSchedulerApi()
	app := cli.NewApp()

	app.Name = "consul-scheduler"
	app.Version = "0.1.0"
	app.Author = "Colin Walker"
	app.Usage = "schedule tasks across a consul cluster."

	RegisterClusterCommands(api, app)
	RegisterTaskCommands(api, app)
	RegisterTaskDefinitionCommands(api, app)
	RegisterServiceCommands(api, app)
	RegisterConfigCommands(api, app)

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
