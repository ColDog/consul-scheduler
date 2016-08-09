package scheduler

import (
	. "github.com/coldog/scheduler/api"
	log "github.com/Sirupsen/logrus"

	"math/rand"
	"fmt"
)

/**

The base scheduler:

This is a simple scheduler for simple workloads. It simply builds up a list of all the tasks that must be placed and then
works down the list to see if they have been placed already. If they haven't, it will place the task on a random machine
that does not have a port conflict and has enough memory / cpu.

*/

func DefaultScheduler(cluster Cluster, api *SchedulerApi) {
	needScheduling := make([]Task, 0)
	newlyAllocatedPorts := make(map[string] []uint)

	hosts, err := api.ListHosts()
	if err != nil {
		log.WithField("error", err).Error("[scheduler] failed")
		return
	}

	for _, serviceName := range cluster.Services {
		service, err := api.GetService(serviceName)
		if err != nil {
			log.WithField("error", err).Error("[scheduler] failed")
			return
		}

		taskDef, err := api.GetTaskDefinition(service.TaskName, service.TaskVersion)
		if err == nil {
			for i := 0; i < service.Desired; i++ {
				needScheduling = append(needScheduling, NewTask(cluster, taskDef, service, i))
			}
		}


		count, err := api.TaskCount(cluster.Name + "_" + service.TaskName)
		if err != nil {
			log.WithField("error", err).Error("[scheduler] failed")
			return
		}

		removed := 0


		tasks, err := api.ListTasks(cluster.Name + "_" + service.TaskName)
		if err != nil {
			log.WithField("error", err).Error("[scheduler] failed")
			return
		}

		for _, remCand := range tasks {
			if remCand.Stopped {
				continue
			}

			if (count - removed) <= service.Min {
				break
			}

			if remCand.TaskDef.Version != service.TaskVersion {
				removed++
				log.WithField("task", remCand.Id()).Warn("[scheduler] removing")
				api.DelTask(remCand)
			}
		}


		if (count - removed) > service.Desired {
			for i := (count - removed) - 1; i > service.Desired - 1; i-- {
				id := fmt.Sprintf("%s_%s_%v-%v", cluster.Name, service.Name, service.TaskVersion, i)
				t, _ := api.GetTask(id)

				if t.Service != "" && !t.Stopped {
					log.WithField("task", id).Warn("[scheduler] removing")
					api.DelTask(t)
				}
			}
		}
	}

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
						if val, ok := newlyAllocatedPorts[cand.Name]; ok {
							task.Port = cand.AvailablePort(val...)
						} else {
							task.Port = cand.AvailablePort()
						}

						if task.Port == 0 {
							log.WithField("task", task.Id()).Error("[scheduler] could not provide a port")
							break
						}
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

	log.Info("[scheduler] success")
}
