package master

import (
	"github.com/coldog/sked/api"
	log "github.com/Sirupsen/logrus"
	"sync"
	"math/rand"
	"time"
	"fmt"
	"cmd/go/testdata/testinternal3"
	"github.com/hashicorp/go-multierror"
)

type DefaultScheduler struct {
	api     api.SchedulerApi
	hosts   map[string]*api.Host
	maxPort map[string]uint
}

func (s *DefaultScheduler) Schedule(cluster *api.Cluster, service *api.Service) error {

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
				if err != nil {
					log.WithField("error", err).Warnf("[scheduler-%s] could not find suitable host", service.Name)
					continue
				}

				t.Host = selectedHost

				selectedPort, err := s.selectPort(t)
				if err != nil {
					// this should realistically never happen
					log.WithField("error", err).Errorf("[scheduler-%s] could not find suitable port", service.Name)
					continue
				}

				// we are good to schedule the task!
				log.WithField("task", t.Id()).Debugf("[scheduler-%s] added task", service.Name)
				t.Port = selectedPort
				s.api.ScheduleTask(t)
				taskMap[t.Id()] = t
			}
		}
	}
	return nil
}

func (s *DefaultScheduler) selectPort(t *api.Task) (uint, error) {
	if t.ProvidePort {
		host := s.hosts[t.Host]

		for _, p := range host.PortSelection {
			if p > s.maxPort[host.Name] {
				return p, nil
			}
		}
	}

	return 0, fmt.Errorf("cannot select a port for this task")
}

func (s *DefaultScheduler) matchHost(maxDisk, maxCpu, maxMem uint64, t *api.Task, cand *api.Host) error {
	if cand.Memory - maxMem < 0 {
		return fmt.Errorf("host does not have enough memory")
	}

	if cand.CpuUnits - maxCpu < 0 {
		return fmt.Errorf("host does not have enough cpu units")
	}

	if cand.DiskSpace - maxDisk < 0 {
		return fmt.Errorf("host does not have enough disk space")
	}

	for _, c := range t.TaskDefinition.Containers {
		if inArray(t.Port, cand.ReservedPorts) {
			return fmt.Errorf("task port is using a reserved port")
		}

		for _, p := range c.GetExecutor().ReservedPorts() {
			if inArray(p, cand.ReservedPorts) {
				return fmt.Errorf("container is using a port reserved by this host")
			}
		}
	}

	return nil
}

func (s *DefaultScheduler) selectHost(t *api.Task) (string, error) {
	var errors *multierror.Error

	rand.Seed(time.Now().Unix())

	var maxDisk uint64
	var maxCpu uint64
	var maxMem uint64

	for _, c := range t.TaskDefinition.Containers {
		maxDisk += c.DiskUse
		maxCpu += c.CpuUnits
		maxMem += c.Memory
	}

	for i := 0; i < len(s.hosts); i++ {
		// todo: change this to be ranked, not random
		sel := rand.Intn(len(s.hosts))
		cand := s.hosts[sel]

		err := s.matchHost(maxDisk, maxCpu, maxMem, t, cand)
		if err != nil {
			multierror.Append(errors, err)
			continue
		}

		return cand.Name, nil
	}

	return "", errors
}

func inArray(item uint, list []uint) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
