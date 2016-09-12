package scheduler

import (
	"github.com/coldog/sked/api"
	"github.com/coldog/sked/tools"

	log "github.com/Sirupsen/logrus"

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
			"":        SpreadRanker,
			"spread":  SpreadRanker,
			"binpack": PackRanker,
		},
	}
}

type DefaultScheduler struct {
	cluster  *api.Cluster
	api      api.SchedulerApi
	hosts    map[string]*api.Host
	maxPort  map[string]uint
	l        *sync.RWMutex
	rankers  map[string]Ranker
}

func (s *DefaultScheduler) SetCluster(c *api.Cluster) {
	s.cluster = c
}

func (s *DefaultScheduler) syncHosts() error {
	s.l.Lock()
	defer s.l.Unlock()

	hosts, err := s.api.ListHosts(&api.HostQueryOpts{ByCluster: s.cluster.Name})
	if err != nil {
		return err
	}

	for _, h := range hosts {
		if h.Draining {
			continue
		}

		s.hosts[h.Name] = h
	}
	return nil
}

func (s *DefaultScheduler) Schedule(d *api.Deployment) error {
	logName := fmt.Sprintf("scheduler:%s-%s", s.cluster.Name, d.Name)

	log.Infof("[%s] starting", logName)

	err := s.syncHosts()
	if err != nil {
		return err
	}

	if len(s.hosts) == 0 {
		return fmt.Errorf("no hosts to schedule on")
	}

	taskDefinition, err := s.api.GetTaskDefinition(d.TaskName, d.TaskVersion)
	if err != nil {
		return err
	}

	tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
		ByCluster: s.cluster.Name,
		ByDeployment: d.Name,
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
		"count":   count,
		"cluster": s.cluster.Name,
		"deployment": d.Name,
		"desired": d.Desired,
	}).Infof("[scheduler-%s] beginning", s.cluster.Name)

	// cleanup, should be implemented by every scheduler
	for key, t := range taskMap {

		// remove the task if the host doesn't exits
		if _, ok := s.hosts[t.Host]; !ok {
			log.WithField("task", t.Id()).Debugf("[%s] removing due to host not existing", logName)
			t.Scheduled = false
			s.api.PutTask(t)
			removed++
			delete(taskMap, key)
		}

		// remove a task if if was rejected
		if t.Rejected {
			log.WithField("reason", t.RejectReason).WithField("error", err).Warnf("[%s] task was rejected", logName)
			t.Scheduled = false
			s.api.PutTask(t)
			removed++
			delete(taskMap, key)
		}

		// If the task is a lower version, and we are still above the min for this service, deschedule the task.
		if (count-removed) > d.Min && t.TaskDefinition.Version != d.TaskVersion {
			log.WithField("task", t.Id()).Debugf("[%s] removing due to lower version", logName)
			t.Scheduled = false
			s.api.PutTask(t)
			removed++
			delete(taskMap, key)
		}
	}

	// if the running count minus the earlier cleanup leaves us with too many running tasks, we remove some.
	if (count - removed) > d.Desired {
		for i := (count - removed) - 1; i > d.Desired-1; i-- {
			id := api.MakeTaskId(s.cluster, d, i)
			if t, ok := taskMap[id]; ok {
				log.WithField("task", t.Id()).Debugf("[%s] removing due to scaling", logName)
				t.Scheduled = false
				s.api.PutTask(t)
				removed++
				delete(taskMap, id)
			}
		}
	}

	// now, move the opposite direction, up to the desired count
	if (count - removed) < d.Desired {
		for i := 0; i < d.Desired; i++ {
			id := api.MakeTaskId(s.cluster, d, i)
			if _, ok := taskMap[id]; !ok {
				// make the task and schedule it
				t := api.NewTask(s.cluster, taskDefinition, d, i)
				selectedHost, err := s.selectHost(d.Scheduler, t)
				if selectedHost == "" {
					log.WithField("error", err).Warnf("[%s] could not find suitable host", logName)
					continue
				}

				t.Host = selectedHost

				for _, c := range t.TaskDefinition.Containers {
					for _, p := range c.Ports {
						if p.Host == 0 {
							selectedPort, err := s.selectPort(t)
							if err != nil {
								// this should realistically never happen
								log.WithField("error", err).Errorf("[%s] could not find suitable port", logName)
								continue
							}

							t.ProvidedPorts[p.Name] = selectedPort
						}
					}
				}

				// we are good to schedule the task!
				log.WithField("host", selectedHost).WithField("task", t.ID()).Debugf("[%s] added task", logName)

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
		"added":   added,
		"total":   count + added - removed,
	}).Infof("[scheduler-%s] done", d.Name)
	return nil
}

func (s *DefaultScheduler) selectPort(t *api.Task) (uint, error) {
	rand.Seed(time.Now().UnixNano())

	s.l.Lock()
	defer s.l.Unlock()

	host := s.hosts[t.Host]

	p := s.maxPort[host.Name] + uint(rand.Intn(2000))
	if p < 9000 {
		// keep a sufficiently high port range, people usually like things in the 8000 range alot...
		p += 9000
	}

	i := 0

	for {
		i++
		if inArray(p, host.ReservedPorts) {
			p += uint(rand.Intn(1000))
			continue
		}

		if p > 65535 {
			p = 8000
			continue
		}

		s.maxPort[host.Name] = p
		return p, nil
	}
}

func (s *DefaultScheduler) matchHost(t *api.Task, cand *api.Host) error {
	counts := t.TaskDefinition.Counts()
	if cand.CalculatedResources.Memory-counts.Memory < 0 {
		return fmt.Errorf("host %s does not have enough memory", cand.Name)
	}

	if cand.CalculatedResources.CpuUnits-counts.CpuUnits < 0 {
		return fmt.Errorf("host %s does not have enough cpu units", cand.Name)
	}

	if cand.CalculatedResources.DiskSpace-counts.DiskUse < 0 {
		return fmt.Errorf("host %s not have enough disk space", cand.Name)
	}

	for _, c := range t.TaskDefinition.Containers {
		for _, p := range c.Ports {
			if p.Host != 0 && inArray(p.Host, cand.ReservedPorts) {
				return fmt.Errorf("task port is using a reserved port %d on host %s", p.Host, cand.Name)
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

	h.CalculatedResources.Memory -= c.Memory
	h.CalculatedResources.DiskSpace -= c.DiskUse
	h.CalculatedResources.CpuUnits -= c.CpuUnits
	h.ReservedPorts = append(h.ReservedPorts, t.TaskDefinition.AllPorts()...)
}

func (s *DefaultScheduler) selectHost(name string, t *api.Task) (string, error) {
	s.l.RLock()
	defer s.l.RUnlock()

	errors := &tools.MultiError{}

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
			errors.Append(err)
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
