package master

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/sked/api"

	"net/http"
	"sync"
	"time"
)

type scheduleReq struct {
	cluster string
	service string
}

func NewMaster(a api.SchedulerApi, conf *Config) *Master {
	return &Master{
		quit:   make(chan struct{}, 1),
		runGc:  make(chan struct{}, 50),
		api:    a,
		queue:  NewSchedulerQueue(),
		locks:  NewSchedulerLocks(a),
		Config: conf,
		schedulers: &Schedulers{
			lock:       &sync.RWMutex{},
			schedulers: make(map[string]Scheduler),
		},
	}
}

type Config struct {
	Runners      int
	SyncInterval time.Duration
	Cluster      string
}

// The scheduler manages scheduling on a per service basis, dispatching requests for scheduling when needed.
type Master struct {
	schedulers *Schedulers
	locks      *SchedulerLocks
	queue      *SchedulerQueue
	Config     *Config
	hostCount  int
	api        api.SchedulerApi
	runGc      chan struct{}
	quit       chan struct{}
}

func (s *Master) Cluster() (*api.Cluster, error) {
	return s.api.GetCluster(s.Config.Cluster)
}

func (s *Master) monitor() {
	log.Info("[master] starting")

	listenConfig := make(chan string, 100)
	s.api.Subscribe("dispatch-config", "config::*", listenConfig)
	defer s.api.UnSubscribe("dispatch")

	listenHealth := make(chan string, 100)
	s.api.Subscribe("dispatch-health", "health::node:failing:*", listenHealth)
	defer s.api.UnSubscribe("dispatch")

	for {
		select {
		case <-listenConfig:
			err := s.dispatch()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}
			s.runGc <- struct {}{}

		case <-listenHealth:
			err := s.dispatch()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}

		case <-time.After(45 * time.Second):
			err := s.dispatch()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}

		case <-s.quit:
			log.Warn("[master] exiting")
			return
		}
	}
}

func (s *Master) dispatch() error {
	cluster, err := s.Cluster()
	if err != nil {
		return err
	}

	for _, serviceName := range cluster.Services {
		err := s.dispatchService(cluster, serviceName)
		if err != nil {
			log.WithField("err", err).Error("[master] error while dispatching service")
		}
	}

	return nil
}

func (s *Master) dispatchService(cluster *api.Cluster, serviceName string) error {
	service, err := s.api.GetService(serviceName)
	if err != nil {
		return err
	}

	tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
		ByCluster: cluster.Name,
		ByService: service.Name,
	})


	unhealthyHost := ""
	for _, task := range tasks {
		ok, err := s.api.AgentHealth(task.Host)
		if err != nil {
			return err
		}

		if !ok {
			unhealthyHost = task.Host
			break
		}
	}

	log.WithFields(log.Fields{
		"count": len(tasks),
		"desired": service.Desired,
		"service": service.Name,
		"cluster": cluster.Name,
		"problem_host": unhealthyHost,
	}).Info("[master] dispatch")

	if unhealthyHost != "" || len(tasks) != service.Desired {
		s.queue.Push(scheduleReq{cluster.Name, service.Name})
	}
	return nil
}

func (s *Master) worker(i int) {
	log.Infof("[master-worker-%d] starting", i)

	for {
		listen := make(chan interface{})
		s.queue.Pop(listen)

		select {
		case item := <-listen:
			val := item.(scheduleReq)
			log.WithField("req", val).Infof("[worker-%d] scheduling", i)
			t1 := time.Now().UnixNano()
			s.schedule(val.cluster, val.service, i)
			t2 := time.Now().UnixNano()

			log.WithFields(log.Fields{
				"time": t2 - t1,
				"cluster": val.cluster,
				"service": val.service,
				"secs": float64(t2 - t1) / 1000000000.0,
			}).Infof("[master-worker-%d] done", i)

		case <-s.quit:
			log.Warnf("[master-worker-%d] exiting", i)
			return
		}

	}

}

func (s *Master) schedule(clusterName, serviceName string, i int) {
	lock, err := s.locks.Lock(serviceName)
	if err != nil {
		log.WithField("service", serviceName).WithField("err", err).Warnf("[master-worker-%d] lock failed", i)
		return
	}
	defer s.locks.Unlock(serviceName)

	if lock.IsHeld() {
		cluster, err := s.api.GetCluster(clusterName)
		if err != nil {
			log.WithField("cluster", clusterName).WithField("err", err).Warnf("[master-worker-%d] could not get cluster", i)
			return
		}

		service, err := s.api.GetService(serviceName)
		if err != nil {
			log.WithField("service", serviceName).WithField("err", err).Warnf("[master-worker-%d] could not get service", i)
			return
		}

		scheduler, ok := s.schedulers.Get(service.Scheduler)
		if !ok {
			log.WithField("scheduler", service.Scheduler).WithField("service", serviceName).Warnf("[master-worker-%d] could not get scheduler", i)
			return
		}

		log.WithField("service", serviceName).Debugf("[master-worker-%d] scheduling", i)
		err = scheduler.Schedule(cluster, service)

		if err != nil {
			log.WithField("service", serviceName).WithField("err", err).Warnf("[master-worker-%d] scheduler errord", i)
		}
	} else {
		log.WithField("service", serviceName).Debugf("[master-worker-%d] could not lock service", i)
	}
}

func (s *Master) garbageCollector() {
	for {
		select {
		case <-s.runGc:
			cluster, err := s.Cluster()
			if err != nil {
				log.WithField("err", err).Error("[master-gc] error while gc getting cluster")
			}

			tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
				ByCluster: s.Config.Cluster,
			})

			if err != nil {
				log.WithField("err", err).Error("[master-gc] error while gc getting tasks")
			}

			for _, task := range tasks {
				if !inArrayStr(task.Service, cluster.Services) {
					s.api.DeScheduleTask(task)
				}
			}

		case <-s.quit:
			log.Warn("[master-gc] exiting")
			return
		}
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
	if s.Config.Runners == 0 {
		s.Config.Runners = 3
	}

	if s.Config.Cluster == "" {
		s.Config.Cluster = "default"
	}

	s.schedulers.Use("", NewDefaultScheduler(s.api))
	for i := 0; i < s.Config.Runners; i++ {
		go s.worker(i)
	}
	s.monitor()
}

func inArrayStr(key string, arr []string) bool {
	for _, v := range arr {
		if v == key {
			return true
		}
	}
	return false
}
