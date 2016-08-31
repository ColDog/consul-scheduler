package actions

import (
	"fmt"
	"github.com/coldog/sked/api"
	"unicode/utf8"
)

func Drain(a api.SchedulerApi, hostName string) error {
	if hostName == "" {
		return fmt.Errorf("no host to scale")
	}

	host, err := a.GetHost(hostName)
	if err != nil {
		return err
	}

	host.Draining = true
	return a.PutHost(host)
}

func Scale(a api.SchedulerApi, serviceName string, desired int) error {
	if serviceName == "" {
		return fmt.Errorf("no service to scale")
	}

	service, err := a.GetService(serviceName)
	if err != nil {
		return err
	}

	if desired < service.Min {
		service.Min = desired
	}

	if desired > service.Max {
		service.Max = desired
	}

	service.Desired = desired
	return a.PutService(service)
}

func ListTasks(a api.SchedulerApi, byHost, byCluster, byService string) error {
	tasks, err := a.ListTasks(&api.TaskQueryOpts{
		ByHost:    byHost,
		ByService: byService,
		ByCluster: byCluster,
		Scheduled: true,
	})

	if err != nil {
		return err
	}

	tableHeader("id", "host", "rejected", "cluster", "service", "task definition", "version")
	for _, t := range tasks {
		tableRow(t.Id(), t.Host, t.Rejected, t.Cluster, t.Service, t.TaskDefinition.Name, t.TaskDefinition.Version)
	}
	return nil
}

func tableHeader(items ...interface{}) {
	r := row(items)
	c := utf8.RuneCountInString(r)
	p := ""
	for i := 0; i < c; i++ {
		p += "-"
	}
	fmt.Println(r)
	fmt.Println(p)
}

func tableRow(items ...interface{}) {
	fmt.Println(row(items))
}

func row(items []interface{}) string {
	s := ""
	for _, i := range items {
		s += pad(fmt.Sprintf("%v", i))
	}
	return s
}

func pad(x string) string {
	c := utf8.RuneCountInString(x)
	for i := 0; i < (20 - c); i++ {
		x += " "
	}
	return x
}
