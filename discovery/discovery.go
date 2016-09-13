package discovery

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"time"
	"sync"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func New(a api.SchedulerApi, managing []string) *DiscoveryAgent {
	return &DiscoveryAgent{
		api: a,
		quit: make(chan struct{}),
		Services: map[string][]*api.Endpoint{},
		ChangeIdx: map[string]int64{},
		Managing: managing,
		changes: make(chan string),
		subs: map[string]chan string{},
		l: &sync.RWMutex{},
	}
}

type DiscoveryAgent struct {
	api      api.SchedulerApi
	quit     chan struct{}
	Services map[string][]*api.Endpoint
	ChangeIdx  map[string]int64
	Managing []string
	changes  chan string
	subs     map[string]chan string
	l        *sync.RWMutex
}

func (ag *DiscoveryAgent) Listen(service string, lastIdx int64, t time.Duration, stop chan struct{}) (int64, []*api.Endpoint) {
	id := tools.RandString(24)
	l := make(chan string)

	ag.l.Lock()
	ag.subs[id] = l
	i := ag.ChangeIdx[service]
	ag.l.Unlock()

	if i > lastIdx {
		ag.l.RLock()
		endpoints := ag.Services[service]
		ag.l.RUnlock()
		return i, endpoints
	}

	for {
		select {
		case n := <-l:
			if n == service {
				ag.l.RLock()
				endpoints := ag.Services[service]
				i := ag.ChangeIdx[service]
				ag.l.RUnlock()
				return i, endpoints
			}

		case <-time.After(t):
			ag.l.RLock()
			endpoints := ag.Services[service]
			i := ag.ChangeIdx[service]
			ag.l.RUnlock()
			return i, endpoints

		case <-stop:
			return 0, nil
		}
	}

}

func (ag *DiscoveryAgent) Stop() {
	close(ag.quit)
}

func (ag *DiscoveryAgent) Run() {
	listener := make(chan string)
	ag.api.Subscribe("dc-agent", "*", listener)
	defer ag.api.UnSubscribe("dc-agent")

	for {
		select {
		case <-listener:
			ag.sync()

		case <-time.After(30 * time.Second):
			ag.sync()

		case <-ag.quit:
			return
		}
	}
}

func (ag *DiscoveryAgent) sync() error {

	var services []*api.Service
	var err error
	if ag.Managing != nil && len(ag.Managing) > 0 {
		services = []*api.Service{}

		for _, s := range ag.Managing {
			ser, err := ag.api.GetService(s)
			if err == nil {
				services = append(services, ser)
			}
		}

	} else {
		services, err = ag.api.ListServices()
	}

	if err != nil {
		return err
	}

	ag.l.Lock()
	defer ag.l.Unlock()

	for _, s := range services {
		endpoints, err := ag.endpoints(s.Name)
		if err != nil {
			return err
		}

		ends, ok := ag.Services[s.Name]
		if !ok {
			ag.Services[s.Name] = endpoints
			ag.changes <- s.Name
			ag.ChangeIdx[s.Name] += 1
			continue
		}

		if len(ends) != len(endpoints) {
			ag.Services[s.Name] = endpoints
			ag.changes <- s.Name
			ag.ChangeIdx[s.Name] += 1
			continue
		}

		for _, e := range endpoints {
			ok := false
			for _, e2 := range ends {
				if e.Address() == e2.Address() {
					ok = true
					break
				}
			}

			if !ok {
				ag.Services[s.Name] = endpoints
				ag.changes <- s.Name
				ag.ChangeIdx[s.Name] += 1
				break
			}
		}

	}

	return nil
}

func (ag *DiscoveryAgent) endpoints(id string) ([]*api.Endpoint, error) {
	endpoints := []*api.Endpoint{}

	service, err := ag.api.GetService(id)
	if err != nil {
		return endpoints, err
	}

	tasks, err := ag.api.ListTasks(&api.TaskQueryOpts{
		Running: true,
		ByDeployment: service.Deployment,
		ByCluster: service.Cluster,
	})

	if err != nil {
		return endpoints, err
	}

	for _, t := range tasks {
		host, err := ag.api.GetHost(t.Cluster, t.Host)
		if err != nil {
			return endpoints, err
		}

		for _, cont := range t.TaskDefinition.Containers {
			if cont.Name == service.Name {
				port := cont.HostPortByName(service.PortName, t)
				if port == 0 {
					// warning
					port = 80
				}

				endpoints = append(endpoints, &api.Endpoint{
					Service: service.Name,
					Port: port,
					Host: host.Address,
					Protocol: service.Protocol,
				})

				break
			}
		}
	}

	return endpoints, nil
}

func (agent *DiscoveryAgent) RegisterRoutes() {
	http.HandleFunc("/discovery/status", func(w http.ResponseWriter, r *http.Request) {
		res, err := json.Marshal(agent)
		if err != nil {
			fmt.Printf("json err: %v\n", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(res)
	})

	http.HandleFunc("/discovery/poll", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		r.ParseForm()

		n := r.Form.Get("service")
		if n == "" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error": "Service Not Provided"}`))
			return
		}

		l, err := strconv.ParseInt(r.Form.Get("idx"), 16, 8)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(400)
			w.Write([]byte(`{"error": "Could not retrieve index"}`))
			return
		}

		stop := make(chan struct{})
		defer close(stop)

		i, end := agent.Listen(n, l, 10 * time.Second, stop)

		res, err := json.Marshal(map[string]interface{} {
			"service": n,
			"endpoints": end,
			"index": i,
		})
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(`{"error": "Failed to return results"}`))
			return
		}

		w.WriteHeader(200)
		w.Write(res)
	})

	http.HandleFunc("/agent/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK\n"))
	})
}
