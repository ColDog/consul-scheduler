package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"

	"fmt"
	"time"
)

func (a *ConsulApi) Subscribe(key, on string, ch chan string) {
	a.eventLock.Lock()
	a.listeners[key] = &listener{on: on, ch: ch}
	a.eventLock.Unlock()
}

func (a *ConsulApi) UnSubscribe(key string) {
	if _, ok := a.listeners[key]; ok {
		a.eventLock.Lock()
		delete(a.listeners, key)
		a.eventLock.Unlock()
	}
}

// should emit events: health::<status (failing|passing)>:<task_id>
func (a *ConsulApi) monitorHealth() {
	lastId := uint64(0)
	for {
		log.WithField("lastId", lastId).Debug("[consul-api] monitoring health")

		checks, meta, err := a.health.State("any", &api.QueryOptions{
			WaitIndex: lastId,
			WaitTime:  3 * time.Minute,
		})

		if err != nil {
			log.WithField("lastId", lastId).WithField("error", err).Debug("[consul-api] monitoring health errored")
			time.Sleep(10 * time.Second)
			continue
		}

		if meta.LastIndex > lastId {
			events := make([]string, 0, len(checks))
			log.WithField("lastId", lastId).Debug("[consul-api] sending health events")
			for _, check := range checks {

				stat := "passing"
				// simplify the statuses, we don't care about critical vs warning
				if check.Status == "critical" || check.Status == "warning" {
					stat = "failing"
				}

				events = append(events, fmt.Sprintf("health::%s:%s", stat, check.ServiceID))
			}
			a.emit(events...)
		} else {
			// have not found any new results
			time.Sleep(10 * time.Second)
		}

		lastId = meta.LastIndex
	}
}

// should emit events: config::<key>
func (a *ConsulApi) monitorConfig() {
	lastId := uint64(0)
	for {
		list, meta, err := a.kv.List(a.conf.ConfigPrefix, &api.QueryOptions{
			WaitIndex: lastId,
		})

		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		if meta.LastIndex > lastId {

			events := make([]string, 0, len(list))
			for _, kv := range list {
				events = append(events, fmt.Sprintf("config::%s", kv.Key))
			}

			log.WithField("lastId", lastId).Debug("[consul-api] sending config events")
			a.emit(events...)
			time.Sleep(2 * time.Second)
		} else {
			// have not found any new results
			time.Sleep(10 * time.Second)
		}

		lastId = meta.LastIndex
	}
}

func (a *ConsulApi) emit(events ...string) {
	a.eventLock.RLock()
	defer a.eventLock.RUnlock()

	for _, listener := range a.listeners {
		for _, evt := range events {
			if match(listener.on, evt) {
				select {
				case listener.ch <- evt:
				default:
				}
				break
			}
		}
	}
}

func match(key, evt string) bool {
	evtR := []rune(evt)
	for idx, char := range key {
		if char == '*' {
			return true
		}

		if idx > (len(evtR) - 1) {
			return false
		}

		if char != evtR[idx] {
			return false
		}
	}

	return true
}
