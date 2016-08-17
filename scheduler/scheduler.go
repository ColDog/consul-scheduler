package scheduler

import (
	log "github.com/Sirupsen/logrus"
	. "github.com/coldog/scheduler/api"

	"errors"
	"fmt"
	"math/rand"
	"time"
	"cmd/go/testdata/testinternal3"
)

var (
	ScheduleTaskFailErr    = errors.New("could not schedule task")
	ScheduleGetHostFailErr = errors.New("could not get hosts")
)

func RunDefaultScheduler(cluster Cluster, api *SchedulerApi, stopCh chan struct{})  {
	s := &DefaultScheduler{
		cluster: cluster,
		api: api,
		stopCh: stopCh,
		maxPort: make(map[string]uint),
	}
	s.Run()
}

// This is a simple scheduler for simple workloads. It simply builds up a list of all the tasks that must be placed and then
// works down the list to see if they have been placed already. If they haven't, it will place the task on a random machine
// that does not have a port conflict and has enough memory / cpu.
type DefaultScheduler struct {
	api     *SchedulerApi
	cluster Cluster
	stopCh  chan struct{}
	hosts   []Host
	maxPort map[string]uint
}

// Runs the scheduler on a specific service. This entails counting the differential between what is running and what
// should be running and writing to the correct keys in consul to make sure that the agents can schedule.
func (scheduler *DefaultScheduler) scheduleForService(service Service) {
	t1 := time.Now().UnixNano()

	// retrieve the task definition for a given service that needs to be scheduled
	taskDef, err := scheduler.api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		log.WithField("error", err).WithField("task", fmt.Sprintf("%s.%d", service.TaskName, service.TaskVersion)).Error("[scheduler] could not find task")
		return
	}

	// build up all of the required variables
	runningName := scheduler.cluster.Name + "_" + service.TaskName
	tasks, err := scheduler.api.ListTasks(runningName)
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed")
		return
	}

	count := len(tasks)
	removed := 0
	added := 0

	log.WithFields(log.Fields{
		"name":    runningName,
		"count":   count,
		"desired": service.Desired,
		"min":     service.Min,
		"max":     service.Max,
	}).Info("[scheduler] scheduling " + service.Name)

	// the next section will remove all of the tasks that shouldn't be running while keeping those running
	// that should be.

	// First of all, we go through and remove tasks (down to the minimum level) that are  running an old
	// version of the task. This is a very specific piece of logic to this scheduler implementation, old
	// tasks are not necessarily evil.
	for _, removalCandidate := range tasks {

		// the tasks can be in a 'stopped' state where they are being kept around in consul for other
		// services to read but they are no longer being actively scheduled by any agents.
		if removalCandidate.Stopped {
			continue
		}

		// don't remove too many.
		if (count - removed) <= service.Min {
			break
		}

		// remove the task if the versions do not match, note this is not 'lower than' to allow rollbacks.
		if removalCandidate.TaskDef.Version != service.TaskVersion {
			removed++
			log.WithField("task", removalCandidate.Id()).Warn("[scheduler] removing")
			scheduler.api.DelTask(removalCandidate)
		}
	}

	// now let's make sure that we aren't over the desired limit by working our way down the list of instances
	// and removing any leftovers.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired-1; i-- {

			// calculate the id that would have been given to this instance
			id := fmt.Sprintf("%s_%s_%v-%v", scheduler.cluster.Name, service.Name, service.TaskVersion, i)
			t, err := scheduler.api.GetTask(id)

			if err == nil && t.Service != "" && !t.Stopped {
				log.WithField("task", id).Warn("[scheduler] removing")
				scheduler.api.DelTask(t)
			}
		}
	}

	// here we should make sure that we have enough instances running, again we do this by calculating the
	// id and checking if the task exists. Any tasks that do not exist, should be scheduled.
	for i := 0; i < service.Desired; i++ {
		id := fmt.Sprintf("%s_%s_%v-%v", scheduler.cluster.Name, service.Name, service.TaskVersion, i)
		if !scheduler.api.IsTaskScheduled(id) {
			scheduler.scheduleTask(service, taskDef, i)
			added++
		}
	}

	t2 := time.Now().UnixNano()
	log.WithFields(log.Fields{"removed": removed, "added": added, "service": service.Name, "time": t2 - t1}).Info("[scheduler] finished scheduling service")

}

// Finds a host for a specific task and if successful, schedules it by placing it into consul.
func (scheduler *DefaultScheduler) scheduleTask(service Service, taskDef TaskDefinition, instance int) error {
	rand.Seed(time.Now().UnixNano())
	task := NewTask(scheduler.cluster, taskDef, service, instance)
	t1 := time.Now().UnixNano()

	for i := 0; i < len(scheduler.hosts); i++ {
		r := rand.Intn(len(scheduler.hosts))
		candidateHost := scheduler.hosts[r]

		if candidateHost.CpuUnits-task.TaskDef.CpuUnits <= 0 && candidateHost.Memory-task.TaskDef.Memory <= 0 {
			log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed selection due to not enough cpu / memory")
			continue
		}

		if task.Port != 0 && inList(task.Port, candidateHost.ReservedPorts) {
			log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed selection due to port conflict")
			continue
		}

		// if we reach here the host is selected!
		task.Host = candidateHost.Name

		if task.TaskDef.ProvidePort {
			task.Port = scheduler.availablePort(candidateHost)

			if task.Port == 0 {
				log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed to select a port")
				continue
			}

			// Setup the health checks in consul with the correc tports
			for _, check := range task.TaskDef.Checks {
				if check.AddProvidedPort {
					if task.TaskDef.ProvidePort && check.Http != "" {
						check.Http = fmt.Sprintf("%s:%d", check.Http, task.Port)
					}

					if task.TaskDef.ProvidePort && check.Tcp != "" {
						check.Http = fmt.Sprintf("%s:%d", check.Tcp, task.Port)
					}
				}
			}
		}

		t2 := time.Now().UnixNano()

		log.WithFields(log.Fields{
			"port": task.Port,
			"host": task.Host,
			"time": t1 - t2,
			"task": task.Id(),
		}).Info("[scheduler] added task")

		scheduler.api.PutTask(task)
		return nil
	}

	log.WithField("task", task.Id()).Warn("[scheduler] failed to find a host")
	return ScheduleTaskFailErr
}

// Finds an available port for a given host, the scheduler will take from the 'PortSelection' array provided by
// the agent which will allow it to select a port that the agent is happy with.
func (scheduler *DefaultScheduler) availablePort(host Host) (sel uint) {

	// To ensure that the scheduler does not issue multiple ports during a scheduling session the 'maxPort' map
	// is used to track the maximum allocated port during this scheduling session per host. If we only choose
	// ports greater than this, we shouldn't run into a conflict. This assumes the 'PortSelection' array is
	// ordered.
	max := scheduler.maxPort[host.Name]

	for _, p := range host.PortSelection {
		if p > max {
			sel = p
		}
	}

	scheduler.maxPort[host.Name] = sel
	return sel
}

func (scheduler *DefaultScheduler) Run() error {
	log.WithField("cluster", scheduler.cluster.Name).Debug("[scheduler] starting")

	hosts, err := scheduler.api.ListHosts()
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed to get hosts")
		return ScheduleGetHostFailErr
	}

	scheduler.hosts = hosts

	for _, serviceName := range scheduler.cluster.Services {

		// read periodically from the stop channel to see if we need to exit.
		select {
		case <-scheduler.stopCh:
			log.Warn("[scheduler] exited prematurely from stop signal")
			return
		default:
		}

		// get the service, an error could mean that the user deleted the service, we
		// will warn of this and then continue to the next service.
		service, err := scheduler.api.GetService(serviceName)
		if err != nil {
			log.WithField("error", err).WithField("service", serviceName).Error("[scheduler] failed to find service")
			continue
		}

		scheduler.scheduleForService(service)
	}

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
