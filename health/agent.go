package health

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/config"

	log "github.com/Sirupsen/logrus"

	"net/http"
	"time"
)

type Config struct {
	AppConfig    *config.Config `json:"app_config"`
}

func NewHealthAgent(a api.SchedulerApi, conf *Config) *HealthAgent {
	return &HealthAgent{
		api: a,
		Config: conf,
		State: make(map[string]*Monitor),

	}
}

type HealthAgent struct {
	Host  string
	Config *Config
	State map[string]*Monitor
	api   api.SchedulerApi
	quit  chan struct{}
}

func (h *HealthAgent) GetHostName() {
	for {
		select {
		case <-h.quit:
			log.Warn("[health] exiting")
			return
		default:
		}

		name, err := h.api.HostName()
		if err == nil {
			h.Host = name
			break
		}

		log.WithField("error", err).Error("[health] could not get host")
		time.Sleep(5 * time.Second)
	}
}

func (h *HealthAgent) Stop() {
	for _, m := range h.State {
		m.Stop()
	}
	close(h.quit)
}

func (h *HealthAgent) RegisterRoutes() {
	http.HandleFunc("/scheduler/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK\n"))
	})
}

func (h *HealthAgent) sync() error {
	tasks, err := h.api.ListTasks(&api.TaskQueryOpts{ByHost: h.Host})
	if err != nil {
		return err
	}

	for _, t := range tasks {
		for _, cont := range t.TaskDefinition.Containers {
			for _, c := range cont.Checks {
				if _, ok := h.State[c.ID]; !ok {
					h.State[c.ID] = NewMonitor(h.api, c, t)
				}
			}
		}
	}

	// garbage collection
	for key, m := range h.State {
		ok := false
		for _, t := range tasks {
			if t.Id() == m.Task.Id() {
				ok = true
				break
			}
		}

		if !ok {
			delete(h.State, key)
		}
	}

	return nil
}

func (h *HealthAgent) Run() {
	h.GetHostName()

	listenState := make(chan string, 50)
	h.api.Subscribe("health-agent", "state::state/hosts/"+h.Host, listenState)
	defer h.api.UnSubscribe("health-agent")
	defer close(listenState)

	for {
		select {
		case <-listenState:
			h.sync()
		case <-time.After(30 * time.Second):
			h.sync()
		case <-h.quit:
			log.Warn("[health] exiting")
			return
		}
	}

}
