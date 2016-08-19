package scheduler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/scheduler/api"

	"errors"
	"sync"
	"time"
)

var (
	NoSchedulerErr = errors.New("No Scheduler Found")
	ApiFailureErr  = errors.New("Api Failure")
)

// The scheduler is a function that takes the cluster it should schedule and a pointer to the api object to do
// some scheduling.
type Scheduler func(cluster *api.Cluster, a api.SchedulerApi, stopCh chan struct{})

func NewMaster(a api.SchedulerApi) *Master {
	m := &Master{
		api:        a,
		Schedulers: make(map[string]Scheduler),
		monitors:   make(map[string]chan struct{}),
		stopCh:     make(chan struct{}),
		schedLock:  &sync.RWMutex{},
		monLock:    &sync.RWMutex{},
	}

	m.Use("default", RunDefaultScheduler)
	return m
}

// The master process registers schedulers on a per cluster basis and for each cluster, attempts to lock the
// responsibility for scheduling on that cluster. This allows for fault tolerant scheduling, as if the scheduler
// is no longer to maintain it's presence, it will fail and another process in the cluster can take over.
// The default scheduler provided is suitable for small workloads, specifically web applications. It focuses on
// being predictable and doesn't care where it's locating a specific workload.
type Master struct {
	api        api.SchedulerApi
	Schedulers map[string]Scheduler
	Default    Scheduler
	monitors   map[string]chan struct{}
	stopCh     chan struct{}
	monLock    *sync.RWMutex
	schedLock  *sync.RWMutex
}

func (master *Master) Use(name string, sched Scheduler) {
	master.schedLock.Lock()
	defer master.schedLock.Unlock()
	log.WithField("scheduler", name).Info("[master] registering scheduler")
	master.Schedulers[name] = sched
}

func (master *Master) monitor(name string, stopCh chan struct{}) {
	log.Infof("[monitor-%s] starting", name)
	defer master.closeMonitor(name)
	defer close(stopCh)

LOCK:
	lock, err := master.api.Lock(master.api.Conf().SchedulersPrefix + name)
	if err != nil {
		log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s]  failed to lock", name)
		return
	}
	defer lock.Unlock()

	lockFailCh, err := lock.Lock(master.stopCh)
	if err != nil {
		log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s]  failed to lock", name)
		return
	}

	listener := make(chan string, 10)
	defer close(listener)

	master.api.Subscribe("monitor-"+name, "config::*", listener)
	defer master.api.UnSubscribe("monitor-" + name)

	err = master.schedule(name, stopCh)
	if err != nil {
		log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s] err scheduling", name)
		return
	}

	for {
		select {
		case <-stopCh:
			log.Warnf("[monitor-%s] exiting due to stoppage", name)
			return

		case <-lockFailCh:
			log.Warnf("[monitor-%s] lock failed", name)
			// attempt to lock again if the lock has failed
			goto LOCK

		case <-listener:
			log.Debugf("[monitor-%s] triggering scheduler", name)
			err := master.schedule(name, stopCh)
			if err != nil {
				log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s] err scheduling", name)
				return
			}

		case <-time.After(30 * time.Second):
			// occasionally poll the api to see if the cluster still exists, sometimes a user may have removed the
			// cluster from the configuration and accordingly this process should exit.
			_, err := master.api.GetCluster(name)
			if err != nil {
				log.WithField("cluster", name).WithField("error", err).Warnf("[monitor-%s] err getting cluster", name)
				return
			}

		}
	}

}

func (master *Master) schedule(name string, stopCh chan struct{}) error {
	// retrieve the cluster, making sure to quick the process if the user has removed it from the
	// configuration.
	cluster, err := master.api.GetCluster(name)
	if err != nil {
		return ApiFailureErr
	}

	scheduler := master.getScheduler(cluster.Scheduler)
	if scheduler == nil {
		return NoSchedulerErr
	} else {
		// the scheduler should run in its own observed process that debounces the schedule
		// requests to regular times.
		go scheduler(cluster, master.api, stopCh)
		return nil
	}
}

func (master *Master) getScheduler(name string) Scheduler {
	master.schedLock.RLock()
	defer master.schedLock.RUnlock()

	if scheduler, ok := master.Schedulers[name]; ok {
		return scheduler
	}
	return nil
}

func (master *Master) closeMonitor(name string) {
	if _, ok := master.monitors[name]; ok {
		master.monLock.Lock()
		delete(master.monitors, name)
		master.monLock.Unlock()
	}
}

func (master *Master) addMonitors() {
	count := 0

	clusters, err := master.api.ListClusters()
	if err != nil {
		log.WithField("error", err).Error("[master] failed to get clusters")
		return
	}

	for _, cluster := range clusters {
		if _, ok := master.monitors[cluster.Name]; !ok {
			count++
			nextStopCh := make(chan struct{})
			go master.monitor(cluster.Name, nextStopCh)
			master.monLock.Lock()
			master.monitors[cluster.Name] = nextStopCh
			master.monLock.Unlock()
		}
	}

	if count > 0 {
		log.WithField("count", count).Info("[master] added monitors")
	}
}

// This process monitors the individual scheduler locking processes. It will start a monitor process if a new cluster
// is added to the configuration.
func (master *Master) monitoring() {
	listener := make(chan string, 10)
	master.api.Subscribe("main-monitor", "config::*", listener)
	defer master.api.UnSubscribe("main-monitor")

	master.addMonitors()

	for {
		select {

		case <-master.stopCh:
			master.monLock.Lock()
			for _, monitorStopCh := range master.monitors {
				close(monitorStopCh)
			}
			master.monLock.Unlock()
			return

		case <-listener:
			master.addMonitors()

		case <-time.After(30 * time.Second):
			master.addMonitors()
		}

	}
}

func (master *Master) Run() {
	log.Info("[master] starting")

	go master.monitoring()

	<-master.stopCh
	log.Warn("[master] exiting")
}

func (master *Master) Stop() {
	close(master.stopCh)
}
