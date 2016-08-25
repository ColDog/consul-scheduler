package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"

	"time"
)

type Config struct {
	Runners      int
	SyncInterval time.Duration
}

// The scheduler manages scheduling on a per service basis, dispatching requests for scheduling when needed.
type Master struct {
	schedulers *Schedulers
	locks      *SchedulerLocks
	queue      *SchedulerQueue
	Config     *Config
	api        api.SchedulerApi
	quit       chan struct{}
}

func (s *Master) monitor() {
	listener := make(chan string, 100)
	s.api.Subscribe("dispatch", "config::*", listener)
	defer s.api.UnSubscribe("dispatch")

	for {
		select {
		case <-listener:
			// todo: set up better logic here
			s.dispatchAll()

		case <-time.After(60 * time.Second):
			s.dispatchAll()

		case <-s.quit:
			return
		}
	}
}

func (s *Master) dispatchAll() {
	services, err := s.api.ListServices()
	if err != nil {
		log.WithField("err", err).Warnf("[master] could not list services")
		return
	}
	for _, service := range services {
		s.queue.Push(service.Name)
	}
}

// todo: add this function to dispatch based on received event
//func (s *Master) dispatch(evt string) {
//	evt = strings.Replace(evt, "config::", "", 1)
//	spl := strings.Split(evt, "/")
//	if spl[1] == "host" {
//
//	} else if spl[1] == "task_definition" {
//		services, _ := s.api.ListServices()
//		for _, service := range services {
//			s.queue.Push(service.Name)
//		}
//
//	} else if spl[1] == "service" {
//		s.queue.Push(spl[1])
//	}
//}

func (s *Master) worker(i int) {
	for {
		listen := make(chan string)
		s.queue.Pop(listen)

		select {
		case serviceName := <-listen:
			s.schedule(serviceName, i)

		case <-s.quit:
			return
		}

	}

}

func (s *Master) schedule(serviceName string, i int) {
	lock, err := s.locks.Lock(serviceName)
	if err != nil {
		log.WithField("service", serviceName).WithField("err", err).Warnf("[worker-%d] lock failed", i)
		return
	}
	defer s.locks.Unlock(serviceName)

	if lock.IsHeld() {
		service, err := s.api.GetService(serviceName)
		if err != nil {
			log.WithField("service", serviceName).WithField("err", err).Warnf("[worker-%d] could not get service", i)
			return
		}

		scheduler, ok := s.schedulers.Get(service.Scheduler)
		if !ok {
			log.WithField("scheduler", service.Scheduler).WithField("service", serviceName).Warnf("[worker-%d] could not get scheduler", i)
			return
		}

		log.WithField("service", serviceName).Debugf("[worker-%d] scheduling", i)
		err = scheduler.Schedule(service, lock.QuitChan())
		if err != nil {
			log.WithField("service", serviceName).WithField("err", err).Warnf("[worker-%d] scheduler errord", i)
		}
	} else {
		log.WithField("service", serviceName).Debugf("[worker-%d] could not lock service", i)
	}
}

func (s *Master) Stop() {
	s.locks.Stop()
	close(s.quit)
}

func (s *Master) Run() {
	for i := 0; i < s.Config.Runners; i++ {
		go s.worker(i)
	}
	s.monitor()
}
