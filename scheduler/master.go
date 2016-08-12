package scheduler

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"
	"github.com/coldog/scheduler/tools"
	"github.com/hashicorp/consul/api"
	"go/internal/gcimporter/testdata"
	"sync"
	"time"
)

/**
 * The master scheduler registers schedulers on a per cluster basis and for each cluster, attempts to lock the
 * responsibility for scheduling on that cluster. This allows for fault tolerant scheduling, as if the scheduler
 * is no longer to maintain it's presence, it will fail and another process in the cluster can take over.
 *
 * The default scheduler provided is suitable for small workloads, specifically web applications. It focuses on
 * being predictable and doesn't care where it's locating a specific workload.
 */

// The scheduler is a function that takes the cluster it should schedule and a pointer to the api object to do
// some scheduling.
type Scheduler func(cluster Cluster, api *SchedulerApi, stopCh chan struct{})

func NewMaster(a *SchedulerApi) *Master {
	m := &Master{
		api:        a,
		Schedulers: make(map[string]Scheduler),
		events:     tools.NewEvents(),
		lock:       &sync.RWMutex{},
		monitors:   make(map[string] chan struct{}),
		stopCh:     make(chan struct{}),
	}

	m.Use("default", DefaultScheduler)
	return m
}

// The master is a process that handles maintaining a lock on each cluster and initiating scheduling when
// required.
type Master struct {
	api        *SchedulerApi
	Schedulers map[string]Scheduler
	lock       sync.RWMutex
	Default    Scheduler
	events     *tools.Events
	monitors   map[string] chan struct{}
	stopCh 	   chan struct{}
}

func (master *Master) Use(name string, sched Scheduler) {
	master.lock.Lock()
	defer master.lock.Unlock()
	log.WithField("scheduler", name).Info("[master] registering scheduler")
	master.Schedulers[name] = sched
}

func (master *Master) monitorCluster(name string, stopCh <-chan struct{}) {
	var err error
	var lock *api.Lock
	var lockFailCh chan struct{}
	var stopLockingCh chan struct{} = make(chan struct{})

	// register a listener with the master's events process. This will send us a message over the listener
	// channel if the configuration is updated.
	listener := make(chan struct{})
	master.events.Subscribe(name, listener)

	defer master.events.UnSubscribe(name)
	defer close(listener)
	defer close(stopLockingCh)
	defer lock.Unlock()

LOCK:
	lock, err = master.api.Lock(SchedulersPrefix + name)
	if err != nil {
		log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s]  failed to lock", name)
		return
	}

	lockFailCh, err = lock.Lock(stopLockingCh)
	if err != nil {
		// this could be because a lock is already held or there are errors with the api, we just exit and let
		// the parent process reschedule this process.
		log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s] failed to lock", name)
		return
	}

	log.WithField("cluster", name).Infof("[monitor-%s] acquired lock", name)

WAIT:
	for {
		select {
		case <-stopCh:
			// read from the stop channel and exit if we need to, note returning will take care of closing
			// all the necessary processes.
			log.Warnf("[monitor-%s] exiting early", name)
			return

		case <-lockFailCh:
			// retry locking if we read from this channel and it fails
			log.WithField("cluster", name).Warnf("[monitor-%s] lock failed", name)
			goto LOCK

		case <-listener:
			// retrieve the cluster, making sure to quick the process if the user has removed it from the
			// configuration.
			cluster, err := master.api.GetCluster(name)
			if err != nil {
				log.WithField("cluster", name).WithField("error", err).Errorf("[monitor-%s] err getting cluster", name)
				return
			}

			scheduler := master.getScheduler(cluster.Scheduler)
			if scheduler == nil {
				log.WithField("cluster", cluster.Name).Warnf("[monitor-%s] no scheduler found", name)
			} else {
				// todo: this should not run in the same process
				// the scheduler should run in its own observed process that debounces the schedule
				// requests to regular times.
				scheduler(cluster, master.api, lockFailCh)
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

func (master *Master) getScheduler(name string) Scheduler {
	master.lock.RLock()
	defer master.lock.RUnlock()
	if scheduler, ok := master.Schedulers[name]; ok {
		return scheduler
	}
	return nil
}

func (master *Master) watcher() {
	for {
		select {
		case <-master.stopCh:
			return
		default:
		}

		log.Debug("[master] waiting")
		err := master.api.WaitOnKey("config/")
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		master.events.Publish()
	}
}

func (master *Master) addMonitors() {
	clusters, err := master.api.ListClusters()
	if err != nil {
		log.WithField("error", err).Error("[master] failed to get clusters")
		return
	}

	for _, cluster := range clusters {
		if _, ok := master.monitors[cluster.Name]; !ok {
			nextStopCh := make(chan struct{})
			go master.monitorCluster(cluster.Name, nextStopCh)
			master.monitors[cluster.Name] = nextStopCh
		}
	}
}

// This process monitors the individual scheduler locking processes. It will start a monitor process if a new cluster
// is added to the configuration.
func (master *Master) monitors() {
	listener := make(chan struct{})
	master.events.Subscribe("main", listener)
	defer master.events.UnSubscribe("main")

	for {
		select {

		case <-master.stopCh:
			for _, monitorStopCh := range master.monitors {
				select {
				case monitorStopCh <- struct {}{}:
				default:
				}
			}
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

	// Loops and checks various api calls to see if consul is initialized. Sometimes if the cluster of consul servers
	// is just starting it will return 500 until a group and a leader is elected.
	for {
		select {
		case <-master.stopCh:
			return
		default:
		}

		_, err := master.api.Host()
		if err == nil {
			break
		}

		_, err = master.api.ListClusters()
		if err == nil {
			break
		}

		log.WithField("error", err).Error("[master] could not find host name")
		time.Sleep(15 * time.Second)
	}

	go master.monitors()
	go master.watcher()

	<-master.stopCh
	log.Info("[master] exiting")
}

func (master *Master) schedule(cluster Cluster, scheduler Scheduler) {
	log.WithField("cluster", cluster.Name).Info("[master] locking scheduler")
	lock, err := master.api.LockScheduler(cluster.Name)
	if err != nil {
		log.WithField("error", err).WithField("cluster", cluster.Name).Error("[master] failed to lock scheduler")
		return
	}

	scheduler(cluster, master.api)
	lock.Unlock()
}
