package cli

import (
	"github.com/coldog/sked/actions"
	"github.com/urfave/cli"
	"strconv"
)

func (app *App) ApplyCmd() (cmd cli.Command) {
	cmd.Name = "apply"
	cmd.Usage = "apply the provided configuration file to consul"
	cmd.Flags = []cli.Flag{
		cli.StringFlag{Name: "file, f", Value: "sked-conf.yml", Usage: "configuration file to read"},
	}
	cmd.Action = func(c *cli.Context) error {
		file := c.String("file")
		return actions.ApplyConfig(file, app.Api)
	}
	return cmd
}

func (app *App) ScaleCmd() (cmd cli.Command) {
	cmd.Name = "scale"
	cmd.Usage = "scale the service"
	cmd.ArgsUsage = "[service] [scale]"
	cmd.Action = func(c *cli.Context) error {
		des := c.Args().Get(1)
		i, err := strconv.ParseInt(des, 8, 64)
		if err != nil {
			return err
		}

		return actions.Scale(app.Api, c.Args().First(), int(i))
	}
	return cmd
}

func (app *App) DrainCmd() (cmd cli.Command) {
	cmd.Name = "drain"
	cmd.Usage = "drain a host of containers"
	cmd.ArgsUsage = "host"
	cmd.Action = func(c *cli.Context) error {
		return actions.Drain(app.Api, c.Args().First())
	}
	return cmd
}


func (app *App) TasksCmd() (cmd cli.Command) {
	cmd.Name = "tasks"
	cmd.Usage = "drain a host of containers"
	cmd.ArgsUsage = "host"
	cmd.Flags = []cli.Flag{
		cli.StringFlag{Name: "by-host", Usage: "filter by host"},
		cli.StringFlag{Name: "by-cluster", Usage: "filter by cluster"},
		cli.StringFlag{Name: "by-service", Usage: "filter by service"},
	}
	cmd.Action = func(c *cli.Context) error {
		return actions.ListTasks(app.Api, c.String("by-host"), c.String("by-cluster"), c.String("by-service"))
	}
	return cmd
}
