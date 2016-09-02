package agent

import (
	"github.com/coldog/sked/api"
	"time"
	"net/http"
	"fmt"
	"net"
	"github.com/coldog/sked/tools"
	log "github.com/Sirupsen/logrus"
)

var checkers = map[string] func(check *api.Check) error {
	"http": checkHTTP,
	"tcp": checkTCP,
	"script": checkScript,
	"docker": checkDocker,
	"none": func(c *api.Check) {
		return fmt.Errorf("no checks")
	},
}

func checkHTTP(c *api.Check) error {
	httpClient := &http.Client{Timeout: c.Timeout}
	resp, err := httpClient.Get(c.HTTP)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status code was %d", resp.StatusCode)
	}

	return err
}

func checkTCP(c *api.Check) error {
	conn, err := net.Dial("tcp", c.TCP)
	defer conn.Close()
	return err
}

func checkScript(c *api.Check) error {
	return tools.Exec(nil, c.Timeout, "sh", "-c", c.Script)
}

func checkDocker(c *api.Check) error {
	return tools.Exec(nil, c.Timeout, "docker", "exec", "-i", c.TaskID, "sh", "-c", c.Script)
}

func NewMonitor(c *api.Check) *Monitor {
	m := &Monitor{
		Check: c,
		Type:  checkType(c),
		quit:  make(chan struct{}),
	}
	go m.Run()
	return m
}

type Monitor struct {
	Failures    int
	LastFailure error
	Status      string
	Failing     bool
	Check       *api.Check
	Type        string
	quit        chan struct{}
	api         api.SchedulerApi
}

func (m *Monitor) Run() {
	for {
		select {

		case <-time.After(m.Check.Interval):

			err := checkers[m.Type](m.Check)

			if err == nil {
				m.Failures = 0
				m.LastFailure = nil
			} else {
				m.Failures += 1
				m.LastFailure = err
			}

			if m.Failures > 3 {
				m.Status = "critical"
			} else if m.Failures > 0 && m.Failures <= 3 {
				m.Status = "warning"
			} else if m.Failures == 0 {
				m.Status = "healthy"
			}

			log.WithField("error", m.LastFailure).WithField("status", m.Status).Infof("[monitor-%s] in status", m.Check.ID)
			err := m.api.PutTaskHealth(m.Check.TaskID, m.Status)
			if err != nil {
				log.WithField("error", err).Warnf("[monitor-%s] errord while checking in", m.Check.ID)
			}

		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) Stop() {
	m.quit <- struct{} {}
}

func checkType(c *api.Check) string {
	if c.HTTP != "" {
		return "http"
	} else if c.TCP != "" {
		return "tcp"
	} else if c.Script != "" {
		return "script"
	} else if c.Docker != "" {
		return "docker"
	} else {
		return "none"
	}
}
