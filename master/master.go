package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"

	"time"
	"sync"
)

// scheduler implements the simple scheduler interface which should be able to handle getting a service and scheduling.
type Scheduler interface {
	Schedule(service *api.Service, quit chan struct{}) error
}

// The scheduler manages scheduling on a per service basis, dispatching requests for scheduling when needed.
type Master struct {
	Schedulers map[string]Scheduler
	api        api.SchedulerApi
	dispatch   chan string
	quit       chan struct{}
	lock       *sync.Mutex
	locks      map[string]Lockable
}

func (s *Master) Monitor() {
	listener := make(chan string, 100)
	s.api.Subscribe("dispatch", "config::*", listener)
	defer s.api.UnSubscribe("dispatch")

	for {
		select {
		case evt := <-listener:

		case <-s.quit:

		case <-time.After(60 * time.Second):

		}
	}
}

func (s *Master) Dispatcher() {
	for serviceName := range s.dispatch {
		service, err := s.api.GetService(serviceName)
		if err != nil {
			log.WithField("service", serviceName).Info("[master] cannot get service")
		}

		scheduler, ok := s.Schedulers[service.Scheduler]
		if ok {
			scheduler.Schedule(service)
		}
	}
}

// lock locks throughout the cluster the right to schedule a given service
func (s *Master) lock(serviceName string) (bool, <-chan struct{}) {

}

// unlock removes the right given to the cluster for scheduling
func (s *Master) unlock(serviceName string) (bool, <-chan struct{}) {

}

// schedule holds a lock on a given service for scheduling and runs the appropriate scheduler
func (s *Master) schedule() {

}
