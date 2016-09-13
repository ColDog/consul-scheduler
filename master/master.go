package master

import (
	log "github.com/Sirupsen/logrus"

	"github.com/coldog/sked/api"
	"github.com/coldog/sked/config"

	"math/rand"
	"net/http"
	"time"
	"github.com/coldog/sked/tools"
	"github.com/coldog/sked/scheduler"
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
		schedulers: scheduler.NewSchedulerMap(),
	}
}

type Config struct {
	Runners      int
	SyncInterval tools.Duration
	Cluster      string
	AppConfig    *config.Config
}

// The scheduler manages scheduling on a per service basis, dispatching requests for scheduling when needed.
type Master struct {
	schedulers *scheduler.Schedulers
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
			err := s.dispatchAll()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}
			s.runGc <- struct{}{}

		case <-listenHealth:
			err := s.dispatchAll()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}

		case <-time.After(60 * time.Second):
			err := s.dispatchAll()
			if err != nil {
				log.WithField("error", err).Warn("[master] failed to dispatch")
			}

		case <-s.quit:
			log.Warn("[master] exiting")
			return
		}
	}
}

func (s *Master) dispatchAll() error {
	cluster, err := s.Cluster()
	if err != nil {
		return err
	}

	for _, name := range cluster.Deployments {
		d, err := s.api.GetDeployment(name)
		if err != nil {
			return err
		}

		s.queue.Push(scheduleReq{cluster.Name, d.Name})
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
			log.WithField("req", val).Infof("[master-worker-%d] scheduling", i)
			t1 := time.Now().UnixNano()
			s.schedule(val.cluster, val.service, i)
			t2 := time.Now().UnixNano()

			log.WithFields(log.Fields{
				"time":    t2 - t1,
				"cluster": val.cluster,
				"service": val.service,
				"secs":    float64(t2-t1) / 1000000000.0,
			}).Infof("[master-worker-%d] done", i)

		case <-s.quit:
			log.Warnf("[master-worker-%d] exiting", i)
			return
		}

	}

}

func (s *Master) schedule(clusterName, depName string, i int) {
	// use a random sleep to allow for locking to work effectively
	rand.Seed(time.Now().Unix())
	time.Sleep(time.Duration(100+rand.Intn(500)) * time.Millisecond)

	l := clusterName + ":" + depName

	lock, err := s.locks.Lock(l)
	if err != nil {
		log.WithField("deployment", depName).WithField("err", err).Warnf("[master-worker-%d] lock failed", i)
		return
	}
	defer s.locks.Unlock(l)

	if lock.IsHeld() {
		cluster, err := s.api.GetCluster(clusterName)
		if err != nil {
			log.WithField("cluster", cluster.Name).WithField("err", err).Warnf("[master-worker-%d] could not get cluster", i)
			return
		}

		deploy, err := s.api.GetDeployment(depName)
		if err != nil {
			log.WithField("deployment", depName).WithField("err", err).Warnf("[master-worker-%d] could not get deployment", i)
			return
		}

		scheduler, ok := s.schedulers.Get(deploy.Scheduler)
		if !ok {
			log.WithField("scheduler", deploy.Scheduler).WithField("deployment", depName).Warnf("[master-worker-%d] could not get scheduler", i)
			return
		}

		log.WithField("deployment", depName).WithField("cluster", clusterName).Infof("[master-worker-%d] scheduling", i)

		scheduler.SetCluster(cluster)
		err = scheduler.Schedule(deploy)
		if err != nil {
			log.WithField("deployment", depName).WithField("err", err).Warnf("[master-worker-%d] scheduler errord", i)
		}
	} else {
		log.WithField("deployment", depName).Debugf("[master-worker-%d] could not lock service", i)
	}
}

func (s *Master) GC() {
	s.runGc <- struct{}{}
}

func (s *Master) garbageCollector() {
	for {
		select {
		case <-s.runGc:
			err := s.runGarbageCollect()
			if err != nil {
				log.WithField("error", err).Error("[master-gc] error while gc")
			}

		case <-s.quit:
			log.Warn("[master-gc] exiting")
			return
		}
	}
}

func (s *Master) runGarbageCollect() error {
	cluster, err := s.Cluster()
	if err != nil {
		return err
	}

	tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
		ByCluster: s.Config.Cluster,
	})

	if err != nil {
		return err
	}

	removed := 0
	for _, task := range tasks {
		if !inArrayStr(task.Deployment, cluster.Deployments) {
			task.Scheduled = false
			s.api.PutTask(task)
			removed++
		}
	}

	log.WithField("removed", removed).Info("[master-gc] ran successfully")
	return nil
}

func (s *Master) RegisterRoutes() {
	http.HandleFunc("/scheduler/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK\n"))
	})
}

func (s *Master) Stop() {
	log.Warn("[master] attemting stop")
	s.locks.Stop()
	close(s.quit)
}

func (s *Master) Use(name string, sked scheduler.Scheduler) {
	s.schedulers.Use(name, sked)
	clus, _ := s.Cluster()
	sked.SetCluster(clus)
}

func (s *Master) Run() {
	if s.Config.Runners == 0 {
		s.Config.Runners = 3
	}

	if s.Config.Cluster == "" {
		s.Config.Cluster = "default"
	}

	if s.Config.SyncInterval.IsNone() {
		s.Config.SyncInterval.Duration = 30 * time.Second
	}

	sked := scheduler.NewDefaultScheduler(s.api)
	s.schedulers.Use("", sked)
	s.schedulers.Use("spread", sked)
	s.schedulers.Use("binpack", sked)

	for i := 0; i < s.Config.Runners; i++ {
		go s.worker(i)
	}

	go s.garbageCollector()
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
