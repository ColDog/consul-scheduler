package main

import (
	_ "net/http/pprof"

	"github.com/coldog/sked/actions"
	"github.com/coldog/sked/agent"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/scheduler"

	log "github.com/Sirupsen/logrus"
	consulApi "github.com/hashicorp/consul/api"
	"github.com/urfave/cli"

	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type AppConfig struct {
	Addr string
	Port int
}

func NewApp() *App {
	app := &App{
		cli: cli.NewApp(),
	}
	app.setup()
	return app
}

type ExitHandler func()
type AppCmd func(app *App) cli.Command

type App struct {
	cli        *cli.App
	Api        api.SchedulerApi
	Config     *AppConfig
	Agent      *agent.Agent
	Master     *scheduler.Master
	atExit     ExitHandler
	ConsulConf *consulApi.Config
}

func (app *App) printWelcome(mode string) {
	fmt.Printf("\nsked starting...\n\n")
	fmt.Printf("          version  %s\n", VERSION)
	fmt.Printf("        log-level  %s\n", log.GetLevel())
	fmt.Printf("       consul-api  %s\n", app.ConsulConf.Address)
	fmt.Printf("        consul-dc  %s\n", app.ConsulConf.Datacenter)
	fmt.Printf("             mode  %s\n", mode)
	fmt.Printf("             pid   %d\n", os.Getpid())
	fmt.Print("\nlog output will now begin streaming....\n")
}

func (app *App) setup() {
	app.ConsulConf = consulApi.DefaultConfig()
	app.Config = &AppConfig{
		Port: 8231,
		Addr: "127.0.0.1",
	}

	app.cli.Name = "sked"
	app.cli.Version = VERSION
	app.cli.Author = "Colin Walker"
	app.cli.Usage = "schedule tasks across a consul cluster."

	app.cli.Flags = []cli.Flag{
		cli.StringFlag{Name: "log, l", Value: "debug", Usage: "log level [debug, info, warn, error]"},
		cli.StringFlag{Name: "consul-api", Value: "", Usage: "consul api"},
		cli.StringFlag{Name: "consul-dc", Value: "", Usage: "consul dc"},
		cli.StringFlag{Name: "consul-token", Value: "", Usage: "consul token"},
	}
	app.cli.Before = func(c *cli.Context) error {
		if c.GlobalString("consul-api") != "" {
			app.ConsulConf.Address = c.GlobalString("consul-api")
		}

		if c.GlobalString("consul-dc") != "" {
			app.ConsulConf.Datacenter = c.GlobalString("consul-dc")
		}

		if c.GlobalString("consul-token") != "" {
			app.ConsulConf.Token = c.GlobalString("consul-token")
		}

		store := api.DefaultStorageConfig()
		app.Api = api.NewConsulApi(store, app.ConsulConf)

		switch c.GlobalString("log-level") {
		case "debug":
			log.SetLevel(log.DebugLevel)
			break
		case "info":
			log.SetLevel(log.InfoLevel)
			break
		case "warn":
			log.SetLevel(log.WarnLevel)
			break
		case "error":
			log.SetLevel(log.ErrorLevel)
			break
		default:
			log.SetLevel(log.DebugLevel)
		}

		return nil
	}

	app.cli.Commands = []cli.Command{
		app.AgentCmd(),
		app.SchedulerCmd(),
		app.CombinedCmd(),
		app.ApplyCmd(),
	}
}

// Registers a listener for SIGTERM and sets the function to run being the 'ExitHandler' function.
func (app *App) AtExit(e ExitHandler) {
	app.atExit = e

	killCh := make(chan os.Signal, 2)
	signal.Notify(killCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-killCh

		fmt.Println(" caught interrupt ")

		// ensure that we will actually quit within 10 seconds, but allow for some
		// cleanup to happen by the code before we exit.
		go func() {
			select {
			case <-killCh:
				log.Fatal("[main] exiting abruptly")
			case <-time.After(5 * time.Second):
				log.Fatal("[main] failed to exit cleanly")
			}
		}()

		app.atExit()
	}()
}

func (app *App) RegisterAgent(c *cli.Context) {
	app.Agent = agent.NewAgent(app.Api, &agent.AgentConfig{
		Port:         app.Config.Port,
		Addr:         app.Config.Addr,
		Runners:      c.Int("agent-runners"),
		SyncInterval: c.Duration("agent-sync-interval"),
	})
}

func (app *App) RegisterMaster(c *cli.Context) {
	app.Master = scheduler.NewMaster(app.Api, &scheduler.MasterConfig{
		SyncInterval:     c.Duration("master-sync-interval"),
		DisabledClusters: c.StringSlice("master-disabled-clusters"),
	})
}

func (app *App) AgentCmd() (cmd cli.Command) {
	cmd.Name = "agent"
	cmd.Usage = "start the agent service"
	cmd.Action = func(c *cli.Context) error {
		app.printWelcome("agent")
		app.Api.Wait()

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
	cmd.Action = func(c *cli.Context) error {
		app.printWelcome("scheduler")
		app.Api.Wait()

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
	cmd.Action = func(c *cli.Context) error {
		app.printWelcome("combined")
		app.Api.Wait()

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

func (app *App) AddCmd(cmd AppCmd) {
	app.cli.Commands = append(app.cli.Commands, cmd(app))
}

func (app *App) Run() {
	app.cli.Run(os.Args)
}

func (app *App) Serve() {
	//http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	//	w.Write([]byte("Consul Scheduler\n"))
	//})

	log.Info("http server starting")
	http.ListenAndServe(fmt.Sprintf(":%d", app.Config.Port), nil)
}

func main() {
	app := NewApp()
	app.Run()
}
