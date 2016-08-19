package scheduler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/coldog/scheduler/api"

	"errors"
	"fmt"
	"math/rand"
	"time"
)

var (
	ErrCouldNotSchedule = errors.New("could not schedule task")
	ErrCouldNotGetHosts = errors.New("could not get hosts")
	ErrExitingEarly     = errors.New("exit signal received")
)

func RunDefaultScheduler(cluster *api.Cluster, a api.SchedulerApi, stopCh chan struct{}) {
	s := &DefaultScheduler{
		cluster: cluster,
		api:     a,
		stopCh:  stopCh,
		maxPort: make(map[string]uint),
	}
	s.Run()
}

// This is a simple scheduler for simple workloads. It simply builds up a list of all the tasks that must be placed and then
// works down the list to see if they have been placed already. If they haven't, it will place the task on a random machine
// that does not have a port conflict and has enough memory / cpu.
type DefaultScheduler struct {
	api     api.SchedulerApi
	cluster *api.Cluster
	stopCh  chan struct{}
	hosts   []*api.Host
	maxPort map[string]uint
}

// Runs the scheduler on a specific service. This entails counting the differential between what is running and what
// should be running and writing to the correct keys in consul to make sure that the agents can schedule.
func (scheduler *DefaultScheduler) scheduleForService(service *api.Service) (int, int, error) {
	t1 := time.Now().UnixNano()

	// retrieve the task definition for a given service that needs to be scheduled
	taskDef, err := scheduler.api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		log.WithField("error", err).WithField("task", fmt.Sprintf("%s.%d", service.TaskName, service.TaskVersion)).Error("[scheduler] could not find task")
		return 0, 0, err
	}

	// build up all of the required variables
	tasks, err := scheduler.api.ListTasks(&api.TaskQueryOpts{
		ByService: scheduler.cluster.Name + "-" + service.Name,
	})
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed")
		return 0, 0, err
	}

	count := len(tasks)
	removed := 0
	added := 0

	log.WithFields(log.Fields{
		"name":    service.Name,
		"count":   count,
		"desired": service.Desired,
		"min":     service.Min,
		"max":     service.Max,
		"hosts":   len(scheduler.hosts),
	}).Info("[scheduler] scheduling " + service.Name)

	// the next section will remove all of the tasks that shouldn't be running while keeping those running
	// that should be.

	// First of all, we go through and remove tasks (down to the minimum level) that are  running an old
	// version of the task. This is a very specific piece of logic to this scheduler implementation, old
	// tasks are not necessarily evil.
	for _, removalCandidate := range tasks {

		// the tasks can be in a 'stopped' state where they are being kept around in consul for other
		// services to read but they are no longer being actively scheduled by any agents.
		if !removalCandidate.Scheduled {
			continue
		}

		// don't remove too many.
		if (count - removed) <= service.Min {
			break
		}

		// remove the task if the versions do not match, note this is not 'lower than' to allow rollbacks.
		if removalCandidate.TaskDefinition.Version != service.TaskVersion {
			removed++
			log.WithField("task", removalCandidate.Id()).Debug("[scheduler] removing")
			scheduler.api.DeScheduleTask(removalCandidate)
		}
	}

	// now let's make sure that we aren't over the desired limit by working our way down the list of instances
	// and removing any leftovers.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired-1; i-- {

			// calculate the id that would have been given to this instance
			id := fmt.Sprintf("%s_%s_%v-%v", scheduler.cluster.Name, service.Name, service.TaskVersion, i)
			t, err := scheduler.api.GetTask(id)

			if err == nil && t.Service != "" && t.Scheduled {
				log.WithField("task", id).Debug("[scheduler] removing")
				scheduler.api.DeScheduleTask(t)
			}
		}
	}

	// here we should make sure that we have enough instances running, again we do this by calculating the
	// id and checking if the task exists. Any tasks that do not exist, should be scheduled.
	for i := 0; i < service.Desired; i++ {
		id := api.MakeTaskId(scheduler.cluster, service, i)

		// if not scheduled
		t, err := scheduler.api.GetTask(id)
		if err != nil || !t.Scheduled {
			err := scheduler.scheduleTask(service, taskDef, i)
			if err != nil && err != ErrCouldNotSchedule {
				log.WithField("error", err).Error("[scheduler] failed")
				return 0, 0, err
			}
			added++
		}
	}

	t2 := time.Now().UnixNano()
	log.WithFields(log.Fields{"removed": removed, "added": added, "service": service.Name, "time": t2 - t1}).Info("[scheduler] finished scheduling service")
	return removed, added, nil
}

// Finds a host for a specific task and if successful, schedules it by placing it into consul.
func (scheduler *DefaultScheduler) scheduleTask(service *api.Service, taskDef *api.TaskDefinition, instance int) error {
	t1 := time.Now().UnixNano()
	rand.Seed(t1)

	// copy this over so we can modify the task definition checks and other attributes
	task := api.NewTask(scheduler.cluster, taskDef, service, instance)

	for i := 0; i < len(scheduler.hosts); i++ {
		r := rand.Intn(len(scheduler.hosts))
		candidateHost := scheduler.hosts[r]

		if candidateHost.CpuUnits-task.TaskDefinition.CpuUnits <= 0 && candidateHost.Memory-task.TaskDefinition.Memory <= 0 {
			log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed selection due to not enough cpu / memory")
			continue
		}

		if task.Port != 0 && inList(task.Port, candidateHost.ReservedPorts) {
			log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed selection due to port conflict")
			continue
		}

		// if we reach here the host is selected!
		task.Host = candidateHost.Name

		if task.TaskDefinition.ProvidePort {
			task.Port = scheduler.availablePort(candidateHost)

			if task.Port == 0 {
				log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed to select a port")
				continue
			}

			// Setup the health checks in consul with the correct ports
			for _, check := range task.TaskDefinition.Checks {
				if check.AddProvidedPort && task.TaskDefinition.ProvidePort {
					if check.HTTP != "" {
						task.Checks = append(task.Checks, &api.Check{
							HTTP: fmt.Sprintf("%s:%d", check.HTTP, task.Port),
							Interval: check.Interval,
						})
					} else if check.TCP != "" {
						task.Checks = append(task.Checks, &api.Check{
							TCP: fmt.Sprintf("%s:%d", check.TCP, task.Port),
							Interval: check.Interval,
						})
					} else {
						task.Checks = append(task.Checks, check)
					}
				}

				fmt.Printf("check: %+v\n", check)
			}
		}

		t2 := time.Now().UnixNano()

		err := scheduler.api.ScheduleTask(task)
		if err != nil {
			return err
		}

		log.WithFields(log.Fields{
			"port": task.Port,
			"host": task.Host,
			"time": t1 - t2,
			"task": task.Id(),
		}).Debug("[scheduler] added task")

		return nil
	}

	log.WithField("task", task.Id()).Warn("[scheduler] failed to find a host")
	return ErrCouldNotSchedule
}

// Finds an available port for a given host, the scheduler will take from the 'PortSelection' array provided by
// the agent which will allow it to select a port that the agent is happy with.
func (scheduler *DefaultScheduler) availablePort(host *api.Host) (sel uint) {

	// To ensure that the scheduler does not issue multiple ports during a scheduling session the 'maxPort' map
	// is used to track the maximum allocated port during this scheduling session per host. If we only choose
	// ports greater than this, we shouldn't run into a conflict. This assumes the 'PortSelection' array is
	// ordered.
	max := scheduler.maxPort[host.Name]

	for _, p := range host.PortSelection {
		if p > max {
			sel = p
			break
		}
	}

	scheduler.maxPort[host.Name] = sel
	return sel
}

func (scheduler *DefaultScheduler) Run() error {
	removed := 0
	added := 0

	t1 := time.Now().UnixNano()
	log.WithField("cluster", scheduler.cluster.Name).Debug("[scheduler] starting")

	hosts, err := scheduler.api.ListHosts()
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed to get hosts")
		return ErrCouldNotGetHosts
	}

	scheduler.hosts = hosts

	for _, serviceName := range scheduler.cluster.Services {

		// read periodically from the stop channel to see if we need to exit.
		select {
		case <-scheduler.stopCh:
			log.Warn("[scheduler] exited prematurely from stop signal")
			return ErrExitingEarly
		default:
		}

		// get the service, an error could mean that the user deleted the service, we
		// will warn of this and then continue to the next service.
		service, err := scheduler.api.GetService(serviceName)
		if err != nil {
			log.WithField("error", err).WithField("service", serviceName).Error("[scheduler] failed to find service")
			continue
		}

		rem, add, _ := scheduler.scheduleForService(service)
		added += add
		removed += rem
	}

	t2 := time.Now().UnixNano()

	log.WithField("time", t2 - t1).WithField("seconds", float64(t2 - t1) / 1000000000.00).WithField("added", added).WithField("removed", removed).Info("[scheduler] finished!")
	return nil
}

func inList(item uint, list []uint) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
