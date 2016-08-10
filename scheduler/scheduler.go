package scheduler

import (
	. "github.com/coldog/scheduler/api"
	log "github.com/Sirupsen/logrus"

	"math/rand"
	"fmt"
	"time"
)

/**

The base scheduler:

This is a simple scheduler for simple workloads. It simply builds up a list of all the tasks that must be placed and then
works down the list to see if they have been placed already. If they haven't, it will place the task on a random machine
that does not have a port conflict and has enough memory / cpu.

*/

func DefaultScheduler(cluster Cluster, api *SchedulerApi) {
	hosts, err := api.ListHosts()
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed")
		return
	}

	log.WithField("hosts", len(hosts)).Debug("[scheduler] found hosts")

	for _, serviceName := range cluster.Services {
		service, err := api.GetService(serviceName)
		if err != nil {
			log.WithField("error", err).Error("[scheduler] failed")
			return
		}

		scheduleForService(api, hosts, cluster, service)
	}
}

func scheduleForService(api *SchedulerApi, hosts []Host, cluster Cluster, service Service)  {
	t1 := time.Now().UnixNano()

	// retrieve the task definition for a given service that needs to be scheduled
	taskDef, err := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		log.WithField("error", err).WithField("task", fmt.Sprintf("%s.%d", service.TaskName, service.TaskVersion)).Error("[scheduler] could not find task")
		return
	}

	// build up all of the required variables
	runningName := cluster.Name + "_" + service.TaskName
	tasks, err := api.ListTasks(runningName)
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed")
		return
	}

	count := len(tasks)
	removed := 0
	added := 0

	log.WithFields(log.Fields{
		"name": runningName,
		"count": count,
		"desired": service.Desired,
		"min": service.Min,
		"max": service.Max,
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
			api.DelTask(removalCandidate)
		}
	}

	// now let's make sure that we aren't over the desired limit by working our way down the list of instances
	// and removing any leftovers.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired - 1; i-- {

			// calculate the id that would have been given to this instance
			id := fmt.Sprintf("%s_%s_%v-%v", cluster.Name, service.Name, service.TaskVersion, i)
			t, err := api.GetTask(id)

			if err == nil && t.Service != "" && !t.Stopped {
				log.WithField("task", id).Warn("[scheduler] removing")
				api.DelTask(t)
			}
		}
	}

	// here we should make sure that we have enough instances running, again we do this by calculating the
	// id and checking if the task exists. Any tasks that do not exist, should be scheduled.
	for i := 0; i < service.Desired; i++ {
		id := fmt.Sprintf("%s_%s_%v-%v", cluster.Name, service.Name, service.TaskVersion, i)
		if !api.IsTaskScheduled(id) {
			scheduleTask(api, hosts, cluster, service, taskDef, i)
			added++
		}
	}

	t2 := time.Now().UnixNano()
	log.WithFields(log.Fields{"removed": removed, "added": added, "service": service.Name, "time": t2 - t1}).Info("[scheduler] finished scheduling service")

}

func scheduleTask(api *SchedulerApi, hosts []Host, cluster Cluster, service Service, taskDef TaskDefinition, instance int)  {
	rand.Seed(time.Now().UnixNano())
	task := NewTask(cluster, taskDef, service, instance)
	t1 := time.Now().UnixNano()

	for i := 0; i < len(hosts); i++ {
		r := rand.Intn(len(hosts))
		candidateHost := hosts[r]

		if candidateHost.CpuUnits - task.TaskDef.CpuUnits <= 0  && candidateHost.Memory - task.TaskDef.Memory <= 0 {
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
			task.Port = availablePort(candidateHost, api)

			if task.Port == 0 {
				log.WithField("host", candidateHost.Name).WithField("task", task.Id()).Debug("[scheduler] failed to select a port")
				continue
			}
		}

		t2 := time.Now().UnixNano()

		log.WithFields(log.Fields{
			"port": task.Port,
			"host": task.Host,
			"time": t1 - t2,
			"task": task.Id(),
		}).Info("[scheduler] added task")

		api.PutTask(task)
		return
	}
}

func availablePort(host Host, api *SchedulerApi) uint {
	reserved := host.ReservedPorts

	tasks, _ := api.ListTasks(StatePrefix + host.Name)
	for _, task := range tasks {
		reserved = append(reserved, task.Port)
	}

	for i := 0; i < len(host.PortSelection); i++ {
		r := rand.Intn(len(host.PortSelection) - 1)
		candidate := host.PortSelection[r]
		if !inList(candidate, reserved) {
			return candidate
		}
	}
	return uint(0)
}

func inList(item uint, list []uint) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}