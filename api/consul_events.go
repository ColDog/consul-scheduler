package api

import (
	"github.com/hashicorp/consul/api"
	"time"
	"fmt"
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
		checks, meta, err := a.health.State("any", &api.QueryOptions{
			WaitIndex: lastId,
		})

		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		lastId = meta.LastIndex

		for _, check := range checks {
			a.emit(fmt.Sprintf("health::%s:%s", check.Status, check.ServiceID))
		}
	}
}

// config:<key>
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

		lastId = meta.LastIndex

		for _, kv := range list {
			a.emit(fmt.Sprintf("config::%s", kv.Key))
		}
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
