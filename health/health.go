// This is an experimental health checking agent.
package health

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

var checkers = map[string]func(check *api.Check, t *api.Task) error{
	"http":   checkHTTP,
	"tcp":    checkTCP,
	"script": checkScript,
	"docker": checkDocker,
	"none": func(c *api.Check, t *api.Task) error {
		return fmt.Errorf("no checks")
	},
}

func checkHTTP(c *api.Check, t *api.Task) error {
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

func checkTCP(c *api.Check, t *api.Task) error {
	conn, err := net.Dial("tcp", c.TCP)
	if err != nil {
		return err
	}
	defer conn.Close()
	return err
}

func checkScript(c *api.Check, t *api.Task) error {
	return tools.Exec(nil, c.Timeout, "sh", "-c", c.Script)
}

func checkDocker(c *api.Check, t *api.Task) error {
	return tools.Exec(nil, c.Timeout, "docker", "exec", "-i", t.Id(), "sh", "-c", c.Docker)
}

func NewMonitor(a api.SchedulerApi, c *api.Check, t *api.Task) *Monitor {

	c.HTTP = strings.Replace(c.HTTP, "$PROVIDED_PORT", fmt.Sprintf("%d", t.Port), 1)
	c.TCP = strings.Replace(c.TCP, "$PROVIDED_PORT", fmt.Sprintf("%d", t.Port), 1)

	m := &Monitor{
		api:   a,
		Check: c,
		Task:  t,
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
	Check       *api.Check
	Task        *api.Task
	Type        string
	quit        chan struct{}
	api         api.SchedulerApi
}

func (m *Monitor) Run() {
	for {
		select {

		case <-time.After(m.Check.Interval):

			err := checkers[m.Type](m.Check, m.Task)

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

			log.WithField("error", m.LastFailure).WithField("status", m.Status).Infof("[monitor:%s] checked", m.Check.ID)

			m.api.PutTaskHealth(m.Task.Id(), m.Status)
			if err != nil {
				log.WithField("error", err).Warnf("[monitor:%s] errord while checking in", m.Check.ID)
			}

		case <-m.quit:
			return
		}
	}
}

func (m *Monitor) Stop() {
	m.quit <- struct{}{}
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
