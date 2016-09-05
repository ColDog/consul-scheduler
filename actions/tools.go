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

	rows := make([][]interface{}, 0, len(tasks))
	for _, t := range tasks {
		ok, err := t.Healthy()
		if err != nil {
			return err
		}

		rows = append(rows, []interface{}{t.Id(), t.Host, t.Rejected, t.Cluster, t.Service, t.TaskDefinition.Name, t.TaskDefinition.Version, ok})
	}

	table([]string{"id", "host", "rejected", "cluster", "service", "task def", "version", "healthy"}, rows)
	return nil
}

func ListHosts(a api.SchedulerApi) error {
	hosts, err := a.ListHosts()

	if err != nil {
		return err
	}
	rows := make([][]interface{}, 0, len(hosts))
	for _, h := range hosts {
		tasks, err := a.ListTasks(&api.TaskQueryOpts{ByHost: h.Name})
		if err != nil {
			return err
		}

		rows = append(rows, []interface{} {h.Name, h.Tags, h.CalculatedResources.CpuUnits, h.CalculatedResources.Memory, h.CalculatedResources.DiskSpace, len(tasks)})
	}
	table([]string{"name", "tags", "cpu", "mem", "disk", "tasks"}, rows)
	return nil
}

func table(header []string, rows [][]interface{})  {
	counts := make([]int, len(header))

	for i := 0; i < len(header); i++ {
		counts[i] = utf8.RuneCountInString(header[i]) + 5

		for _, row := range rows {
			val := fmt.Sprintf("%v", row[i])
			c := utf8.RuneCountInString(val) + 5

			if c > counts[i] {
				counts[i] = c
			}
		}
	}

	head := row(header, counts)
	c := utf8.RuneCountInString(head)
	p := ""
	for i := 0; i < c; i++ {
		p += "-"
	}

	fmt.Println("")
	fmt.Println(head)
	fmt.Println(p)

	for _, r := range rows {
		ro := make([]string, 0, len(r))
		for _, ra := range r {
			ro = append(ro, fmt.Sprintf("%v", ra))
		}
		fmt.Println(row(ro, counts))
	}
}

func row(items []string, counts []int) string {
	s := ""
	for i, item := range items {
		s += pad(item, counts[i])
	}
	return s
}

func pad(x string, padding int) string {
	c := utf8.RuneCountInString(x)
	for i := 0; i < (padding - c); i++ {
		x += " "
	}
	return x
}
