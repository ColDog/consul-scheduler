package discovery

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	"time"
	"sync"
)

type DiscoveryAgent struct {
	api       api.SchedulerApi
	quit      chan struct{}
	services  map[string][]*api.Endpoint
	changes   chan string
	subs      map[string]chan string
	l         *sync.RWMutex
}

func (ag *DiscoveryAgent) Subscribe(ch chan string) string {
	id := tools.RandString(24)

	ag.l.Lock()
	defer ag.l.Unlock()

	ag.subs[id] = ch
	return id
}

func (ag *DiscoveryAgent) UnSubscribe(id string) {
	ag.l.Lock()
	defer ag.l.Unlock()

	if _, ok := ag.subs[id]; ok {
		delete(ag.subs, id)
	}
}

func (ag *DiscoveryAgent) Run() {
	listener := make(chan string)
	ag.api.Subscribe("dc-agent", "*", listener)
	defer ag.api.UnSubscribe("dc-agent")

	for {
		select {
		case <-listener:
			ag.Sync()

		case <-time.After(30 * time.Second):
			ag.Sync()

		case <-ag.quit:
			return
		}
	}
}

func (ag *DiscoveryAgent) Sync() error {
	services, err := ag.api.ListServices()
	if err != nil {
		return err
	}

	ag.l.Lock()
	defer ag.l.Unlock()

	for _, s := range services {
		endpoints, err := ag.ListEndpoints(s.Name)
		if err != nil {
			return err
		}

		ends, ok := ag.services[s.Name]
		if !ok {
			ag.services[s.Name] = endpoints
			ag.changes <- s.Name
			continue
		}

		if len(ends) != len(endpoints) {
			ag.services[s.Name] = endpoints
			ag.changes <- s.Name
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
				ag.services[s.Name] = endpoints
				ag.changes <- s.Name
				break
			}
		}

	}
}

func (ag *DiscoveryAgent) ListEndpoints(id string) ([]*api.Endpoint, error) {
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
