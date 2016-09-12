package scheduler

import (
	"github.com/coldog/sked/api"
	"sync"
)

// scheduler implements the simple scheduler interface which should be able to handle getting a service and scheduling.
type Scheduler interface {
	Schedule(d *api.Deployment) error
	SetCluster(c *api.Cluster)
}

type Ranker func(hosts map[string]*api.Host) []RankedHost

type RankedHost struct {
	Name  string
	Score int
}

func NewSchedulerMap() *Schedulers {
	return &Schedulers{
		lock:       &sync.RWMutex{},
		schedulers: make(map[string]Scheduler),
	}
}

type Schedulers struct {
	schedulers map[string]Scheduler
	lock       *sync.RWMutex
}

func (s *Schedulers) Get(name string) (Scheduler, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	sked, ok := s.schedulers[name]
	return sked, ok
}

func (s *Schedulers) Use(name string, sked Scheduler) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.schedulers[name] = sked
}
