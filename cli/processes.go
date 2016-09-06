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
		cli.BoolFlag{Name: "agent-check-health", Usage: "start the health checker"},
		cli.StringFlag{Name: "run-consul", Usage: "start the consul agent with these flags (not for production use)"},
	}
	cmd.Action = func(c *cli.Context) error {
		if c.String("run-consul") != "" {
			app.StartConsul(c.String("run-consul"))
		}

		app.printWelcome("agent")

		app.Api.Start()

		app.RegisterAgent(c)
		app.AtExit(func() {
			app.Agent.Stop()
		})

		app.Agent.RegisterRoutes()
		go app.Serve()
		app.Agent.Run()
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
	}
	cmd.Action = func(c *cli.Context) error {
		if c.String("run-consul") != "" {
			app.StartConsul(c.String("run-consul"))
		}

		app.printWelcome("combined")

		app.Api.Start()

		app.RegisterMaster(c)
		app.RegisterAgent(c)

		app.Master.RegisterRoutes()
		app.Agent.RegisterRoutes()

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

		app.AtExit(func() {
			app.Agent.Stop()
			app.Master.Stop()
		})

		wg.Wait()
		return nil
	}
	return cmd
}
