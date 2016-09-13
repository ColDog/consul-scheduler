// This is an experimental health checking agent.
package health

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"net"
	"net/http"
	"time"
)

type HealthCheck struct {
	TaskID   string
	TCP      string
	HTTP     string
	Script   string
	Docker   string
	DockerID string
	Timeout  time.Duration
	Interval time.Duration
}

type HealthChecker func(c *HealthCheck)

var checkers = map[string]func(c *HealthCheck) error{
	"http":   checkHTTP,
	"tcp":    checkTCP,
	"script": checkScript,
	"docker": checkDocker,
	"none": func(c *HealthCheck) error {
		return fmt.Errorf("no checks")
	},
}

func checkHTTP(c *HealthCheck) error {
	httpClient := &http.Client{Timeout: c.Timeout}
	resp, err := httpClient.Get(c.HTTP)

	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status code was %d", resp.StatusCode)
	}

	return err
}

func checkTCP(c *HealthCheck) error {
	conn, err := net.Dial("tcp", c.TCP)
	if err != nil {
		return err
	}
	defer conn.Close()
	return err
}

func checkScript(c *HealthCheck) error {
	return tools.Exec(nil, c.Timeout, "sh", "-c", c.Script)
}

func checkDocker(c *HealthCheck) error {
	return tools.Exec(nil, c.Timeout, "docker", "exec", "-i", c.DockerID, "sh", "-c", c.Docker)
}

func NewMonitor(a api.SchedulerApi, ch *HealthCheck) *Monitor {
	m := &Monitor{
		api:   a,
		Check: ch,
		Type:  checkType(ch),
		quit:  make(chan struct{}),
	}
	go m.Run()
	return m
}

type Monitor struct {
	Failures    int
	LastFailure error
	Status      api.TaskState
	Check       *HealthCheck
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
				m.Status = api.FAILING
			} else if m.Failures > 0 && m.Failures <= 3 {
				m.Status = api.WARNING
			} else if m.Failures == 0 {
				m.Status = api.RUNNING
			}

			log.WithField("error", m.LastFailure).WithField("status", m.Status).Infof("[monitor:%s] checked", m.Check.TaskID)

			m.api.PutTaskState(m.Check.TaskID, m.Status)
			if err != nil {
				log.WithField("error", err).Warnf("[monitor:%s] errord while checking in", m.Check.TaskID)
			}

		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) Stop() {
	m.quit <- struct{}{}
}

func checkType(c *HealthCheck) string {
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
