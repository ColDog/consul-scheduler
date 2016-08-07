package scheduler

import (
	. "github.com/coldog/scheduler/api"
	log "github.com/Sirupsen/logrus"

	"math/rand"
)

/**

The base scheduler:

This is a simple scheduler for simple workloads. It simply builds up a list of all the tasks that must be placed and then
works down the list to see if they have been placed already. If they haven't, it will place the task on a random machine
that does not have a port conflict and has enough memory / cpu.

*/

func DefaultScheduler(cluster Cluster, api *SchedulerApi) {
	needScheduling := make([]Task, 0)

	for _, serviceName := range cluster.Services {
		service, ok := api.GetService(serviceName)
		if ok {
			taskDef, ok := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
			if ok {
				for i := 0; i < service.Desired; i++ {
					needScheduling = append(needScheduling, NewTask(cluster, taskDef, service, i))
				}
			}

		}
	}

	newlyAllocatedPorts := make(map[string] []uint)

	hosts := api.ListHosts()
	for _, task := range needScheduling {
		scheduled := false

		if !api.IsTaskScheduled(task.Id()) {

			for i := 0; i < len(hosts); i++ {
				r := rand.Intn(len(hosts))
				cand := hosts[r]


				isPortAvailable := cand.IsPortAvailable(task.Port)

				portAlreadyAlloc := false
				if val, ok := newlyAllocatedPorts[cand.Name]; ok {
					for _, p := range val {
						if p == task.Port {
							portAlreadyAlloc = true
							break
						}
					}
				}

				if isPortAvailable && !portAlreadyAlloc && cand.CpuUnits - task.TaskDef.CpuUnits > 0 && cand.Memory - task.TaskDef.Memory > 0 {
					// everything looks good!

					task.Host = cand.Name
					if task.TaskDef.ProvidePort {
						task.Port = cand.AvailablePort()
						println("providing port", task.Port)
					}

					if task.Port != 0 {
						newlyAllocatedPorts[cand.Name] = append(newlyAllocatedPorts[cand.Name], task.Port)
					}
					api.PutTask(task)
					scheduled = true
					break
				}

			}

			if !scheduled {
				log.WithField("task", task.Id()).Error("[scheduler] no hosts found")
			}
		}
	}
}
