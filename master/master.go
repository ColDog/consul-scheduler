package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"

	"time"
	"sync"
	"encoding/json"
	"fmt"
	"net/http"
)

type scheduleReq struct {
	cluster string
	service string
}

func NewMaster(a api.SchedulerApi, conf *Config) *Master {
	return &Master{
		quit: make(chan struct{}, 1),
		api: a,
		queue: NewSchedulerQueue(),
		locks: NewSchedulerLocks(a),
		Config: conf,
		schedulers: Schedulers{
			lock: sync.RWMutex{},
			schedulers: make(map[string]Scheduler),
		},
	}
}

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
	clusters, err := s.api.ListClusters()
	if err != nil {
		log.WithField("err", err).Warnf("[master] could not list services")
		return
	}
	for _, c := range clusters {
		for _, ser := range c.Services {
			s.queue.Push(scheduleReq{c.Name, ser})
		}
	}
}

func (s *Master) worker(i int) {
	for {
		listen := make(chan interface{})
		s.queue.Pop(listen)

		select {
		case item := <-listen:
			val := item.(scheduleReq)
			s.schedule(val.cluster, val.service, i)

		case <-s.quit:
			return
		}

	}

}

func (s *Master) schedule(clusterName, serviceName string, i int) {
	lock, err := s.locks.Lock(serviceName)
	if err != nil {
		log.WithField("service", serviceName).WithField("err", err).Warnf("[worker-%d] lock failed", i)
		return
	}
	defer s.locks.Unlock(serviceName)

	if lock.IsHeld() {
		cluster, err := s.api.GetCluster(clusterName)
		if err != nil {
			log.WithField("cluster", clusterName).WithField("err", err).Warnf("[worker-%d] could not get cluster", i)
			return
		}

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
		err = scheduler.Schedule(cluster, service)

		if err != nil {
			log.WithField("service", serviceName).WithField("err", err).Warnf("[worker-%d] scheduler errord", i)
		}
	} else {
		log.WithField("service", serviceName).Debugf("[worker-%d] could not lock service", i)
	}
}

func (s *Master) RegisterRoutes() {
	http.HandleFunc("/master/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK\n"))
	})
}

func (s *Master) Stop() {
	s.locks.Stop()
	close(s.quit)
}

func (s *Master) Use(name string, sked Scheduler) {
	s.schedulers.Use(name, sked)
}

func (s *Master) Run() {
	s.schedulers.Use("", &DefaultScheduler{})
	for i := 0; i < s.Config.Runners; i++ {
		go s.worker(i)
	}
	s.monitor()
}
