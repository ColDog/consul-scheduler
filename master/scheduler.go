package master

import (
	"github.com/coldog/sked/api"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/go-multierror"

	"fmt"
	"math/rand"
	"sync"
	"time"
)

func NewDefaultScheduler(a api.SchedulerApi) *DefaultScheduler {
	return &DefaultScheduler{
		api:     a,
		hosts:   make(map[string]*api.Host),
		maxPort: make(map[string]uint),
		l:       &sync.RWMutex{},
		rankers: map[string]Ranker{
			"": SpreadRanker,
			"spread": SpreadRanker,
			"binpack": PackRanker,
		},
	}
}

type DefaultScheduler struct {
	api      api.SchedulerApi
	hosts    map[string]*api.Host
	maxPort  map[string]uint
	l        *sync.RWMutex
	lastSync time.Time
	rankers  map[string]Ranker
}

func (s *DefaultScheduler) syncHosts() error {
	s.l.Lock()
	defer s.l.Unlock()

	s.lastSync = time.Now()
	hosts, err := s.api.ListHosts()
	if err != nil {
		return err
	}

	for _, h := range hosts {
		ok, err := s.api.AgentHealth(h.Name)
		if err != nil {
			return err
		}

		if h.Draining {
			continue
		}

		if ok {
			s.hosts[h.Name] = h
		}
	}
	return nil
}

func (s *DefaultScheduler) Schedule(name string, cluster *api.Cluster, service *api.Service) error {
	log.Infof("[scheduler-%s] starting", cluster.Name)

	err := s.syncHosts()
	if err != nil {
		return err
	}

	if len(s.hosts) == 0 {
		return fmt.Errorf("no hosts to schedule on")
	}

	taskDefinition, err := s.api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		return err
	}

	tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
		ByCluster: cluster.Name,
		ByService: service.Name,
		Scheduled: true,
	})

	if err != nil {
		return err
	}

	taskMap := make(map[string]*api.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.Id()] = t
	}

	count := len(tasks)
	removed := 0
	added := 0

	log.WithFields(log.Fields{
		"count": count,
		"service": service.Name,
		"cluster": cluster.Name,
		"desired": service.Desired,
	}).Infof("[scheduler-%s] beginning", cluster.Name)

	// cleanup, should be implemented by every scheduler
	for key, t := range taskMap {

		// remove the task if the host doesn't exits
		if _, ok := s.hosts[t.Host]; !ok {
			t.Scheduled = false
			s.api.PutTask(t)
			removed--
			delete(taskMap, key)
		}

		// remove a task if if was rejected
		if t.Rejected {
			log.WithField("reason", t.RejectReason).WithField("error", err).Warnf("[scheduler-%s] task was rejected", service.Name)
			t.Scheduled = false
			s.api.PutTask(t)
			removed--
			delete(taskMap, key)
		}

		// If the task is a lower version, and we are still above the min for this service, deschedule the task.
		if (count-removed) > service.Min && t.TaskDefinition.Version != service.TaskVersion {
			t.Scheduled = false
			s.api.PutTask(t)
			removed--
			delete(taskMap, key)
		}
	}

	// if the running count minus the earlier cleanup leaves us with too many running tasks, we remove some.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired-1; i-- {
			id := api.MakeTaskId(cluster, service, i)
			if t, ok := taskMap[id]; ok {
				t.Scheduled = false
				s.api.PutTask(t)
				removed--
				delete(taskMap, id)
			}
		}
	}

	// now, move the opposite direction, up to the desired count
	if (count - removed) < service.Desired {
		for i := 0; i < service.Desired; i++ {
			id := api.MakeTaskId(cluster, service, i)
			if _, ok := taskMap[id]; !ok {
				// make the task and schedule it
				t := api.NewTask(cluster, taskDefinition, service, i)
				selectedHost, err := s.selectHost(name, t)
				if selectedHost == "" {
					log.WithField("error", err).Warnf("[scheduler-%s] could not find suitable host", service.Name)
					continue
				}

				t.Host = selectedHost

				if t.TaskDefinition.Port != 0 {
					t.Port = t.TaskDefinition.Port
				}

				if t.TaskDefinition.ProvidePort {
					selectedPort, err := s.selectPort(t)
					if err != nil {
						// this should realistically never happen
						log.WithField("error", err).Errorf("[scheduler-%s] could not find suitable port", service.Name)
						continue
					}
					t.Port = selectedPort
				}

				// we are good to schedule the task!
				log.WithField("host", selectedHost).WithField("port", t.Port).WithField("task", t.Id()).Debugf("[scheduler-%s] added task", service.Name)

				t.Scheduled = true

				s.updateHost(t)

				s.api.PutTask(t)
				added++
				taskMap[t.Id()] = t
			}
		}
	}

	log.WithFields(log.Fields{
		"removed": removed,
		"added": added,
		"total": count + added - removed,
	}).Infof("[scheduler-%s] done", service.Name)
	return nil
}

func (s *DefaultScheduler) selectPort(t *api.Task) (uint, error) {
	s.l.Lock()
	defer s.l.Unlock()

	host := s.hosts[t.Host]

	for _, p := range host.PortSelection {
		if p > s.maxPort[host.Name] {
			s.maxPort[host.Name] = p
			return p, nil
		}
	}

	// take a guess if we cannot get a port from the provided portSelection list.
	p := s.maxPort[host.Name] + 1
	if p == 1 {
		p = 20000
	}

	for {
		if inArray(p, host.ReservedPorts) {
			p += 1
			continue
		}

		s.maxPort[host.Name] = p
	}
	return p, nil
}

func (s *DefaultScheduler) matchHost(t *api.Task, cand *api.Host) error {
	counts := t.TaskDefinition.Counts()
	if cand.Memory-counts.Memory < 0 {
		return fmt.Errorf("host does not have enough memory")
	}

	if cand.CpuUnits-counts.CpuUnits < 0 {
		return fmt.Errorf("host does not have enough cpu units")
	}

	if cand.DiskSpace-counts.DiskUse < 0 {
		return fmt.Errorf("host does not have enough disk space")
	}

	for _, c := range t.TaskDefinition.Containers {
		if inArray(t.Port, cand.ReservedPorts) {
			return fmt.Errorf("task port is using a reserved port")
		}

		for _, p := range c.GetExecutor().ReservedPorts() {
			if inArray(p, cand.ReservedPorts) {
				return fmt.Errorf("container is using a port reserved by this host")
			}
		}
	}

	return nil
}

func (s *DefaultScheduler) updateHost(t *api.Task) {
	s.l.Lock()
	defer s.l.Unlock()

	c := t.TaskDefinition.Counts()

	h := s.hosts[t.Host]

	h.Memory -= c.Memory
	h.DiskSpace -= c.DiskUse
	h.CpuUnits -= c.CpuUnits
	h.ReservedPorts = append(h.ReservedPorts, t.AllPorts()...)
}

func (s *DefaultScheduler) selectHost(name string, t *api.Task) (string, error) {
	s.l.RLock()
	defer s.l.RUnlock()

	var errors *multierror.Error

	rand.Seed(time.Now().Unix())

	ranker, ok := s.rankers[name]
	if !ok {
		ranker = s.rankers[""]
	}

	ranking := ranker(s.hosts)
	for _, r := range ranking {
		cand, ok := s.hosts[r.Name]
		if !ok {
			continue
		}

		err := s.matchHost(t, cand)
		if err != nil {
			multierror.Append(errors, err)
			continue
		}

		return cand.Name, nil
	}

	return "", errors
}

func inArray(item uint, list []uint) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
