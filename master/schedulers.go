package master

import (
	"github.com/coldog/sked/api"
	"sync"
)

// scheduler implements the simple scheduler interface which should be able to handle getting a service and scheduling.
type Scheduler interface {
	Schedule(cluster *api.Cluster, service *api.Service) error
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
