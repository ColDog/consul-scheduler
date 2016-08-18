package api

import (
	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"

	"fmt"
	"time"
)

func (a *ConsulApi) Listen(evt string, listener chan string) {
	a.eventLock.Lock()
	a.listeners[evt] = listener
	a.eventLock.Unlock()
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
			log.WithField("lastId", lastId).Debug("[consul-api] sending health events")
			for _, check := range checks {

				stat := "passing"
				// simplify the statuses, we don't care about critical vs warning
				if check.Status == "critical" || check.Status == "warning" {
					stat = "failing"
				}

				fmt.Printf("check: %+v\n", check)

				a.emit(fmt.Sprintf("health::%s:%s", stat, check.ServiceID))
			}
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
			log.WithField("lastId", lastId).Debug("[consul-api] sending config events")
			a.emit("config::change")
			for _, kv := range list {
				a.emit(fmt.Sprintf("config::%s", kv.Key))
			}
		} else {
			// have not found any new results
			time.Sleep(10 * time.Second)
		}

		lastId = meta.LastIndex
	}
}

func (a *ConsulApi) emit(evt string) {
	for key, listener := range a.listeners {
		if match(key, evt) {
			select {
			case listener <- evt:
			default:
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
