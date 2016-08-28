package master

import (
	"github.com/coldog/sked/api"
	log "github.com/Sirupsen/logrus"
	"sync"
	"math/rand"
	"time"
)

type DefaultScheduler struct {
	api     api.SchedulerApi
	hosts   map[string]*api.Host
	maxPort map[string]uint
}

func (s *DefaultScheduler) Schedule(cluster *api.Cluster, service *api.Service, quit <-chan struct{}) error {

	taskDefinition, err := s.api.GetTaskDefinition(service.TaskName, service.TaskVersion)
	if err != nil {
		return err
	}

	tasks, err := s.api.ListTasks(&api.TaskQueryOpts{
		ByCluster: cluster.Name,
		ByService: service.Name,
	})

	if err != nil {
		return err
	}

	taskMap := make(map[string]*api.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.Id()] = t
	}

	count := len(tasks)
	removed := 0

	// cleanup, should be implemented by every scheduler realistically
	for key, t := range taskMap {

		// garbage collection, remove tasks from hosts that don't exist
		host, err := s.api.GetHost(t.Host)
		if err != api.ErrNotFound {
			return err
		}
		if err == api.ErrNotFound || host == nil {
			s.api.DeScheduleTask(t)
			removed--
			delete(taskMap, key)
		}

		// If the task is a lower version, and we are still above the min for this service, deschedule the task.
		if (count - removed) > service.Min && t.TaskDefinition.Version != service.TaskVersion {
			s.api.DeScheduleTask(t)
			removed--
			delete(taskMap, key)
		}
	}

	// if the running count minus the earlier cleanup leaves us with too many running tasks, we remove some.
	if (count - removed) > service.Desired {
		for i := (count - removed) - 1; i > service.Desired-1; i-- {
			id := api.MakeTaskId(cluster, service, i)
			if t, ok := taskMap[id]; ok {
				s.api.DeScheduleTask(t)
				removed--
				delete(taskMap, id)
			}
		}
	}

	// now, move the opposite direction, up to the desired count
	if (count - removed) < service.Desired {
		for i := 0; i < service.Desired; i++ {
			id := api.MakeTaskId(cluster, service, i)
			if _, ok := taskMap[id]; !ok {
				// make the task and schedule it
				t := api.NewTask(cluster, taskDefinition, service, i)
				selectedHost, err := s.selectHost(t)
				if err == nil {
					t.Host = selectedHost
					s.selectPort(t)

					s.api.ScheduleTask(t)
					taskMap[t.Id()] = t
				} else {
					log.WithField("error", err).Warnf("[scheduler-%s] could not find suitable host", service.Name)
				}
			}
		}
	}
	return nil
}

func (s *DefaultScheduler) selectPort(t *api.Task) {

}

// given a candidate task select a new host to use.
func (s *DefaultScheduler) selectHost(t *api.Task) (string, error) {
	rand.Seed(time.Now().Unix())

	for i := 0; i < len(s.hosts); i++ {
		sel := rand.Intn(len(s.hosts))


	}
}
