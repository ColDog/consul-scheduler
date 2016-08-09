package scheduler

import (
	. "github.com/coldog/scheduler/api"
	log "github.com/Sirupsen/logrus"
	"time"
)

type Scheduler func(cluster Cluster, api *SchedulerApi)

func NewMaster(a *SchedulerApi) *Master {
	log.Info("[master] starting master process")

	m := &Master{
		api: a,
		Schedulers: make(map[string] Scheduler),
	}

	m.Use("default", DefaultScheduler)
	return m
}

type Master struct {
	api 			*SchedulerApi
	Schedulers 		map[string] Scheduler
	Default			Scheduler
}

func (master *Master) Use(name string, sched Scheduler) {
	log.WithField("scheduler", name).Info("[master] registering scheduler")
	master.Schedulers[name] = sched
}

func (master *Master) Run() {
	log.Info("[master] running")

	for {
		log.Info("[master] waiting")
		master.api.WaitOnKey("config/")

		log.Info("[master] starting schedulers")

		clusters, err := master.api.ListClusters()
		if err != nil {
			log.WithField("error", err).Error("[master] failed to get clusters")
			continue
		}

		if len(clusters) == 0 {
			log.Info("[master] nothing to schedule")
		}

		for _, cluster := range clusters {
			scheduler, ok := master.Schedulers[cluster.Scheduler]
			if ok {
				go master.schedule(cluster, scheduler)
			} else {
				log.WithField("cluster", cluster.Name).Warn("[master] no scheduler found")
			}
		}

	}
}

func (master *Master) schedule(cluster Cluster, scheduler Scheduler) {
	t1 := time.Now().UnixNano()

	log.WithField("cluster", cluster.Name).Info("[master] locking scheduler")
	lock, err := master.api.LockScheduler(cluster.Name)
	if err != nil {
		log.WithField("error", err).WithField("cluster", cluster.Name).Error("[master] failed to lock scheduler")
		return
	}

	scheduler(cluster, master.api)
	lock.Unlock()

	t2 := time.Now().UnixNano()
	log.WithField("cluster", cluster.Name).WithField("time", t2 - t1).Info("[master] finished scheduling")
}