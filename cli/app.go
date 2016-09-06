package cli

import (
	_ "net/http/pprof"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/coldog/sked/agent"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/config"
	"github.com/coldog/sked/master"

	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"os/exec"
	"github.com/coldog/sked/health"
	"github.com/coldog/sked/tools"
)

func init() {
	log.SetFormatter(&log.TextFormatter{})
}

func NewApp() *App {
	app := &App{
		cli:    cli.NewApp(),
		Config: config.NewConfig(),
	}
	app.setup()
	return app
}

type AppCmd func(app *App) cli.Command

type App struct {
	cli    *cli.App
	Api    api.SchedulerApi
	Config *config.Config
	Agent  *agent.Agent
	Master *master.Master
	Health *health.HealthAgent
	atExit func()
}

func (app *App) printWelcome(mode string) {
	fmt.Printf("\nsked starting...\n\n")
	fmt.Printf("          version  %s\n", config.VERSION)
	fmt.Printf("        log-level  %s\n", log.GetLevel())
	fmt.Printf("       consul-api  %s\n", app.Config.ConsulConfig.Address)
	fmt.Printf("        consul-dc  %s\n", app.Config.ConsulConfig.Datacenter)
	fmt.Printf("             mode  %s\n", mode)
	fmt.Printf("              pid  %d\n", os.Getpid())
	fmt.Printf("           server  %s:%d\n", app.Config.Addr, app.Config.Port)
	fmt.Printf("        advertise  %s\n", app.Config.Advertise)
	fmt.Print("\nlog output will now begin streaming....\n")
}

func (app *App) setup() {
	app.cli.Name = "sked"
	app.cli.Version = config.VERSION
	app.cli.Author = "Colin Walker"
	app.cli.Usage = "schedule tasks across a consul cluster."

	app.cli.Flags = []cli.Flag{
		cli.StringFlag{Name: "log-level, l", Value: "debug", EnvVar: "LOG_LEVEL", Usage: "log level [debug, info, warn, error]"},
		cli.StringFlag{Name: "backend", Value: "consul", EnvVar: "BACKEND", Usage: "storage backend to use"},
		cli.StringFlag{Name: "consul-api", Value: "", EnvVar: "CONSUL_API", Usage: "consul api"},
		cli.StringFlag{Name: "consul-dc", Value: "", EnvVar: "CONSUL_DC", Usage: "consul dc"},
		cli.StringFlag{Name: "consul-token", Value: "", EnvVar: "CONSUL_TOKEN", Usage: "consul token"},
		cli.StringFlag{Name: "bind, b", EnvVar: "BIND", Value: "0.0.0.0", Usage: "address to bind to"},
		cli.StringFlag{Name: "advertise, a", EnvVar: "ADVERTISE", Value: "127.0.0.1:8231", Usage: "address to advertise"},
		cli.IntFlag{Name: "port, p", EnvVar: "PORT", Value: 8231, Usage: "port to bind to"},
	}
	app.cli.Before = func(c *cli.Context) error {
		app.Config.Port = c.GlobalInt("port")
		app.Config.Addr = c.GlobalString("bind")
		app.Config.Advertise = c.GlobalString("advertise")
		app.Config.LogLevel = c.GlobalString("log-level")
		app.Config.Backend = config.Backend(c.GlobalString("backend"))

		if c.GlobalString("consul-api") != "" {
			app.Config.ConsulConfig.Address = c.GlobalString("consul-api")
		}

		if c.GlobalString("consul-dc") != "" {
			app.Config.ConsulConfig.Datacenter = c.GlobalString("consul-dc")
		}

		if c.GlobalString("consul-token") != "" {
			app.Config.ConsulConfig.Token = c.GlobalString("consul-token")
		}

		// todo: allow override of storage config
		store := api.DefaultStorageConfig()

		if app.Config.Backend == config.CONSUL {
			app.Api = api.NewConsulApi(store, app.Config.ConsulConfig)
		} else if app.Config.Backend == config.MEMORY {
			app.Api = api.NewMockApi()
		} else {
			log.Fatalf("backend %s not supported", app.Config.Backend)
		}

		switch app.Config.LogLevel {
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
		app.ScaleCmd(),
		app.DrainCmd(),
		app.TasksCmd(),
		app.HostsCmd(),
	}
}

// Registers a listener for SIGTERM and sets the function to run being the 'ExitHandler' function.
func (app *App) AtExit(e func()) {
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
		Runners:      c.Int("agent-runners"),
		SyncInterval: tools.Duration{c.Duration("agent-sync-interval")},
		AppConfig:    app.Config,
		Resources: &api.Resources{
			Memory:    c.Int64("memory"),
			DiskSpace: c.Int64("disk-space"),
			CpuUnits:  c.Int64("cpu-units"),
		},
	})
}

func (app *App) RegisterHealthAgent(c *cli.Context) {
	app.Health = health.NewHealthAgent(app.Api, &health.Config{
		AppConfig:    app.Config,
	})
}

func (app *App) RegisterMaster(c *cli.Context) {
	app.Master = master.NewMaster(app.Api, &master.Config{
		SyncInterval: tools.Duration{c.Duration("scheduler-sync-interval")},
		Runners:      c.Int("scheduler-runners"),
		Cluster:      c.String("scheduler-cluster"),
		AppConfig:    app.Config,
	})
}

// starts up a consul server
func (app *App) StartConsul(flags string) {
	fmt.Printf("starting consul: %s\n", "consul agent " + flags)
	cmd := exec.Command("sh", "-c", "consul agent " + flags)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Start()
}

func (app *App) Stats() map[string]interface{} {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return map[string]interface{}{
		"version": config.VERSION,
		"name":    "sked",
		"go":      runtime.Version(),
		"runtime": map[string]interface{}{
			"goroutines":    runtime.NumGoroutine(),
			"alloc":         mem.Alloc,
			"total_alloc":   mem.TotalAlloc,
			"heap_alloc":    mem.HeapAlloc,
			"heap_sys":      mem.HeapSys,
			"heap_released": mem.HeapReleased,
			"heap_objects":  mem.HeapObjects,
			"cgo_calls":     runtime.NumCgoCall(),
			"num_cpu":       runtime.NumCPU(),
		},
	}
}

func (app *App) AddCmd(cmd AppCmd) {
	app.cli.Commands = append(app.cli.Commands, cmd(app))
}

func (app *App) Run() {
	app.cli.Run(os.Args)
}

func (app *App) Serve() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Sked\n"))
	})

	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		res, err := json.Marshal(app.Stats())
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
	})

	log.Info("http server starting")
	http.ListenAndServe(fmt.Sprintf(":%d", app.Config.Port), nil)
}
