package cli

import (
	"github.com/coldog/sked/actions"
	"github.com/urfave/cli"
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
	cmd.ArgsUsage = "service"
	cmd.Flags = []cli.Flag{
		cli.IntFlag{Name: "desired, d", Usage: "service count"},
	}
	cmd.Action = func(c *cli.Context) error {
		return actions.Scale(app.Api, c.Args().First(), c.Int("desired"))
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
