package scheduler

import (
	. "github.com/coldog/scheduler/api"
	log "github.com/Sirupsen/logrus"
	"time"
)

type Scheduler func(cluster Cluster, api *SchedulerApi)

func NewMaster(a *SchedulerApi) *Master {
	log.Info("[master] starting master process")

	return &Master{
		api: a,
		Schedulers: make(map[string] Scheduler),
		Default: DefaultScheduler,
	}
}

type Master struct {
	api 			*SchedulerApi
	Schedulers 		map[string] Scheduler
	Default			Scheduler
}

func (master *Master) Register(name string, sched Scheduler) {
	log.WithField("scheduler", name).Info("[master] registering scheduler")
	master.Schedulers[name] = sched
}

func (master *Master) Run() {
	log.Info("[master] wait for trigger")

	for {
		master.api.WaitForScheduler()

		lock := master.api.LockScheduler()
		log.Info("[master] acquired scheduler lock")

		master.schedule()

		master.api.FinishedScheduling()
		lock.Unlock()
		log.Info("[master] unlocked scheduler lock")
	}
}

func (master *Master) schedule() {
	log.Info("[master] beginning scheduler")
	t1 := time.Now().UnixNano()

	for _, cluster := range master.api.ListClusters() {
		scheduler, ok := master.Schedulers[cluster.Scheduler]
		if !ok {
			scheduler = master.Default
		}

		scheduler(cluster, master.api)
	}

	t2 := time.Now().UnixNano()
	log.WithField("time", t2 - t1).Info("[master] finished scheduling")
}
