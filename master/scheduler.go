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
	}
}

type DefaultScheduler struct {
	api      api.SchedulerApi
	hosts    map[string]*api.Host
	maxPort  map[string]uint
	l        *sync.RWMutex
	lastSync time.Time
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

		if ok {
			s.hosts[h.Name] = h
		}
	}
	return nil
}

func (s *DefaultScheduler) Schedule(cluster *api.Cluster, service *api.Service) error {
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

	// cleanup, should be implemented by every scheduler realistically
	for key, t := range taskMap {

		// garbage collection, remove tasks from hosts that don't exist
		host, err := s.api.GetHost(t.Host)
		if err != api.ErrNotFound {
			return err
		}
		if err == api.ErrNotFound || host == nil {
			s.api.DeScheduleTask(t)
			removed--
			delete(taskMap, key)
		}

		// If the task is a lower version, and we are still above the min for this service, deschedule the task.
		if (count-removed) > service.Min && t.TaskDefinition.Version != service.TaskVersion {
			s.api.DeScheduleTask(t)
			removed--
			delete(taskMap, key)
		}
	}

	// if the running count minus the earlier cleanup leaves us with too many running tasks, we remove some.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired-1; i-- {
			id := api.MakeTaskId(cluster, service, i)
			if t, ok := taskMap[id]; ok {
				s.api.DeScheduleTask(t)
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
				selectedHost, err := s.selectHost(t)
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
				s.api.ScheduleTask(t)
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
	s.maxPort[host.Name] = p
	return p, nil
}

func (s *DefaultScheduler) matchHost(maxDisk, maxCpu, maxMem uint64, t *api.Task, cand *api.Host) error {
	if cand.Memory-maxMem < 0 {
		return fmt.Errorf("host does not have enough memory")
	}

	if cand.CpuUnits-maxCpu < 0 {
		return fmt.Errorf("host does not have enough cpu units")
	}

	if cand.DiskSpace-maxDisk < 0 {
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

func (s *DefaultScheduler) selectHost(t *api.Task) (string, error) {
	s.l.RLock()
	defer s.l.RUnlock()

	var errors *multierror.Error

	rand.Seed(time.Now().Unix())

	var maxDisk uint64
	var maxCpu uint64
	var maxMem uint64

	for _, c := range t.TaskDefinition.Containers {
		maxDisk += c.DiskUse
		maxCpu += c.CpuUnits
		maxMem += c.Memory
	}

	// todo: should be ranked
	for _, cand := range s.hosts {

		err := s.matchHost(maxDisk, maxCpu, maxMem, t, cand)
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
