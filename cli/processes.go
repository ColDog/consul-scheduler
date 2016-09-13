package cli

import (
	"github.com/urfave/cli"

	"sync"
	"time"
)

func (app *App) AgentCmd() (cmd cli.Command) {
	cmd.Name = "agent"
	cmd.Usage = "start the agent service"
	cmd.Flags = []cli.Flag{
		cli.DurationFlag{Name: "agent-sync-interval", Value: 30 * time.Second, Usage: "interval to sync agent"},
		cli.IntFlag{Name: "agent-runners", Value: 3, Usage: "amount of tasks to start in parallel on the agent"},
		cli.BoolFlag{Name: "health-agent", Usage: "start the health checker"},
		cli.StringFlag{Name: "run-consul", Usage: "start the consul agent with these flags (not for production use)"},
	}
	cmd.Action = func(c *cli.Context) error {
		if c.String("run-consul") != "" {
			app.StartConsul(c.String("run-consul"))
		}

		app.printWelcome("agent")

		app.Api.Start()

		app.AtExit(func() {
			app.Agent.Stop()
			if app.Health != nil {
				app.Health.Stop()
			}
		})


		if c.Bool("health-agent") {
			app.RegisterHealthAgent(c)
			app.Health.RegisterRoutes()
			go app.Health.Run()
		}

		app.RegisterAgent(c)
		app.Agent.RegisterRoutes()

		go app.Serve()

		app.Agent.Run()

		return nil
	}
	return cmd
}

func (app *App) HealthAgentCmd() (cmd cli.Command) {
	cmd.Name = "health-agent"
	cmd.Usage = "start the health service"
	cmd.Action = func(c *cli.Context) error {
		app.printWelcome("health-agent")

		app.Api.Start()

		app.AtExit(func() {
			app.Health.Stop()
		})

		app.RegisterHealthAgent(c)

		app.Health.RegisterRoutes()
		go app.Serve()
		app.Health.Run()
		return nil
	}
	return cmd
}

func (app *App) DiscoveryAgentCmd() (cmd cli.Command) {
	cmd.Name = "discovery"
	cmd.Usage = "start the discovery service"
	cmd.Action = func(c *cli.Context) error {
		app.printWelcome("discovery-agent")

		app.Api.Start()

		app.AtExit(func() {
			app.Discovery.Stop()
		})

		app.RegisterDiscoveryAgent(c)

		app.Discovery.RegisterRoutes()
		go app.Serve()
		app.Discovery.Run()
		return nil
	}
	return cmd
}


func (app *App) SchedulerCmd() (cmd cli.Command) {
	cmd.Name = "scheduler"
	cmd.Usage = "start the scheduler service"
	cmd.Flags = []cli.Flag{
		cli.DurationFlag{Name: "scheduler-sync-interval", Value: 30 * time.Second, Usage: "interval to sync schedulers"},
		cli.IntFlag{Name: "scheduler-runners", Usage: "amount of schedulers to run in parallel"},
		cli.StringFlag{Name: "scheduler-cluster", Usage: "the cluster to monitor for scheduling"},
		cli.StringFlag{Name: "run-consul", Usage: "start the consul agent with these flags (not for production use)"},
	}
	cmd.Action = func(c *cli.Context) error {
		if c.String("run-consul") != "" {
			app.StartConsul(c.String("run-consul"))
		}

		app.printWelcome("scheduler")

		app.Api.Start()

		app.RegisterMaster(c)
		app.AtExit(func() {
			app.Master.Stop()
		})

		app.Master.RegisterRoutes()
		go app.Serve()
		app.Master.Run()
		return nil
	}
	return cmd
}

func (app *App) CombinedCmd() (cmd cli.Command) {
	cmd.Name = "combined"
	cmd.Usage = "start the scheduler and agent service"
	cmd.Flags = []cli.Flag{
		cli.DurationFlag{Name: "agent-sync-interval", Value: 30 * time.Second, Usage: "interval to sync agent"},
		cli.IntFlag{Name: "agent-runners", Value: 3, Usage: "amount of tasks to start in parallel on the agent"},
		cli.DurationFlag{Name: "scheduler-sync-interval", Value: 30 * time.Second, Usage: "interval to sync schedulers"},
		cli.IntFlag{Name: "scheduler-runners", Usage: "amount of schedulers to run in parallel"},
		cli.StringFlag{Name: "scheduler-cluster", Usage: "the cluster to monitor for scheduling"},
		cli.BoolFlag{Name: "agent-check-health", Usage: "start the health checker"},
		cli.StringFlag{Name: "run-consul", Usage: "start the consul agent with these flags (not for production use)"},
		cli.BoolFlag{Name: "health-agent", Usage: "start the health checker"},
	}
	cmd.Action = func(c *cli.Context) error {
		if c.String("run-consul") != "" {
			app.StartConsul(c.String("run-consul"))
		}

		app.printWelcome("combined")

		app.Api.Start()

		app.AtExit(func() {
			app.Agent.Stop()
			app.Master.Stop()
			if app.Health != nil {
				app.Health.Stop()
			}
		})

		app.RegisterMaster(c)
		app.RegisterAgent(c)

		app.Master.RegisterRoutes()
		app.Agent.RegisterRoutes()

		if c.Bool("health-agent") {
			app.RegisterHealthAgent(c)
			app.Health.RegisterRoutes()
		}

		go app.Serve()

		wg := &sync.WaitGroup{}

		wg.Add(2)
		go func() {
			app.Agent.Run()
			wg.Done()
		}()

		go func() {
			app.Master.Run()
			wg.Done()
		}()

		if c.Bool("health-agent") {
			wg.Add(1)
			go func() {
				app.Health.Run()
				wg.Done()
			}()
		}

		wg.Wait()
		return nil
	}
	return cmd
}
