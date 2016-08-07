package main

import (
	"github.com/urfave/cli"
	. "github.com/coldog/scheduler/api"
	. "github.com/coldog/scheduler/cli"
	. "github.com/coldog/scheduler/agent"
	. "github.com/coldog/scheduler/scheduler"

	"os"
	"time"
)

func GetConfig(c *cli.Context) *Config {
	conf := &Config{}

	conf.ConsulApiAddress = c.GlobalString("address")
	conf.ConsulApiDc = c.GlobalString("datacenter")
	conf.ConsulApiToken = c.GlobalString("token")
	conf.ConsulApiWaitTime = c.GlobalDuration("wait-time")

	return conf
}

func main() {
	var api *SchedulerApi
	app := cli.NewApp()

	app.Version = "0.1.0"
	app.Author = "Colin Walker"
	app.EnableBashCompletion = true

	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "address", Usage: "consul api address"},
		cli.StringFlag{Name: "datacenter", Usage: "consul api datacenter"},
		cli.StringFlag{Name: "token", Usage: "consul api token"},
		cli.DurationFlag{Name: "wait-time", Usage: "consul api wait time", Value: 10 * time.Minute},
	}

	app.Before = func(c *cli.Context) error {
		api = NewSchedulerApi()
		return nil
	}

	RegisterClusterCommands(api, app)
	RegisterTaskCommands(api, app)
	RegisterTaskDefinitionCommands(api, app)
	RegisterClusterCommands(api, app)

	app.Commands = append(app.Commands, cli.Command{
		Name: "start",
		Action: func(c *cli.Context) error {

			m := NewMaster(api)
			ag := NewAgent(api)

			go m.Run()
			ag.Run()

			return nil
		},
	})

	app.Run(os.Args)
}
