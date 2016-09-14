package etcd

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"

	"strings"
	"fmt"
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/backends"
)

type listener struct {
	on string
	ch chan string
}

func (a *EtcdApi) Subscribe(key, on string, ch chan string) {
	a.eventLock.Lock()
	a.listeners[key] = &listener{on: on, ch: ch}
	a.eventLock.Unlock()
}

func (a *EtcdApi) UnSubscribe(key string) {
	if _, ok := a.listeners[key]; ok {
		a.eventLock.Lock()
		delete(a.listeners, key)
		a.eventLock.Unlock()
	}
}

// Listen for custom events emitted from the API,
// can match events using a * pattern.
// Events that should be emitted on change of a key:
// => health::task:<node>:<status>:<task_id>
// => hosts::<host_id>
// => config::<resource (service|task_definition|host|cluster)>/<resource_id>
// => state::<host_id>:<task_id>
func (a *EtcdApi) watch(dir string) {
	watcher := a.kv.Watcher(a.prefix+dir, &client.WatcherOptions{
		Recursive: true,
	})
	for {
		res, err := watcher.Next(context.Background())
		if err != nil {
			log.WithField("err", err).Warn("[etcd-api] watcher failing")
			continue
		}

		k := res.Node.Key

		if strings.HasPrefix(k, a.prefix+"config/") {
			spl := strings.Split(k, "/")
			resource := spl[len(spl)-2]
			id := spl[len(spl)-1]
			a.emit(fmt.Sprintf("config::%s:%s", resource, id))

		} else if strings.HasPrefix(k, a.prefix+"hosts/") {
			spl := strings.Split(k, "/")
			a.emit(fmt.Sprintf("hosts::%s", spl[len(spl)-1]))

		} else if strings.HasPrefix(k, a.prefix+"state/health/") {
			status := res.Node.Value
			taskId := fromPath(k, "health")
			t, err := a.GetTask(taskId)
			if err != nil {
				log.WithField("error", err).WithField("id", taskId).Warn("[etcd-api] getting task errored")
				continue
			}

			a.emit(fmt.Sprintf("health::task:%s:%s:%s", t.Host, status, t.ID()))

		} else if strings.HasPrefix(k, a.prefix+"state/tasks/") {
			t := &api.Task{}
			backends.Decode([]byte(res.Node.Value), t)

			a.emit(fmt.Sprintf("state::%s:%s", t.Host, t.ID()))
		}

	}
}

func (a *EtcdApi) emit(events ...string) {
	a.eventLock.RLock()
	defer a.eventLock.RUnlock()

	for _, listener := range a.listeners {
		for _, evt := range events {
			if match(listener.on, evt) {
				select {
				case listener.ch <- evt:
				default:
				}
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

func fromPath(path, name string) string {
	spl := strings.Split(path, "/")
	for i, s := range spl {
		if s == name {
			if i + 1 < len(spl) {
				return spl[i+1]
			}
		}
	}
	return ""
}
