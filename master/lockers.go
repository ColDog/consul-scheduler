package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"
	"sync"
)

func NewSchedulerLocks(a api.SchedulerApi) *SchedulerLocks {
	return &SchedulerLocks{
		locks: make(map[string]api.Lockable),
		lock:  &sync.Mutex{},
		api:   a,
	}
}

type SchedulerLocks struct {
	locks map[string]api.Lockable
	lock  *sync.Mutex
	api   api.SchedulerApi
}

// lock locks throughout the cluster the right to schedule a given service
func (s *SchedulerLocks) Lock(serviceName string) (locker api.Lockable, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	locker, ok := s.locks[serviceName]
	if !ok {
		locker, err = s.api.Lock("schedulers/"+serviceName)
		if err != nil {
			return locker, err
		}
		_, err := locker.Lock()
		s.locks[serviceName] = locker
		if err != nil {
			return locker, err
		}
	} else {
		_, err := locker.Lock()
		if err != nil {
			return locker, err
		}
	}

	return locker, nil
}

// unlock removes the right given to the cluster for scheduling
func (s *SchedulerLocks) Unlock(serviceName string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	locker, ok := s.locks[serviceName]
	log.WithField("lock", serviceName).WithField("ok", ok).Debug("[master-lockers] unlocking")
	if ok {
		locker.Unlock()
	}
}

func (s *SchedulerLocks) Stop() {
	for _, locker := range s.locks {
		locker.Unlock()
	}
}
