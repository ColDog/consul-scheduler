package api

import (
	"fmt"
	"testing"
	"time"
)

func TestMatcher(t *testing.T) {
	if !match("test:*", "test:event") {
		t.Fatal("test:event should match to test:*")
	}

	if match("testing:*", "test:event") {
		t.Fatal("test:event should not match to testing:*")
	}
}

func TestHealthEventWaiting(t *testing.T) {
	agent := NewConsulAgent()
	defer agent.Stop()

	api := newConsulApi()
	for {
		_, err := api.HostName()
		if err == nil {
			break
		}
	}

	api.Start()

	listener := make(chan string)
	api.Subscribe("test-health", "health::*", listener)
	defer api.UnSubscribe("test-health")

	c := 0
	go func() {
		for val := range listener {
			fmt.Printf("event: %s\n", val)
			c++
		}
	}()

	time.Sleep(15 * time.Second)

	if c == 0 {
		t.Fatal("could not get any events from health monitor")
	}
}

func TestConfigEventWaiting(t *testing.T) {
	agent := NewConsulAgent()
	defer agent.Stop()

	api := newConsulApi()
	for {
		_, err := api.HostName()
		if err == nil {
			break
		}
	}

	api.Start()

	listener := make(chan string, 30)
	api.Subscribe("test-config", "*", listener)
	defer api.UnSubscribe("test-config")

	c := 0
	go func() {
		for val := range listener {
			fmt.Printf("event: %s\n", val)
			c++
		}
	}()

	go func() {
		time.Sleep(5 * time.Second)
		fmt.Println("---> events sending")
		api.PutCluster(SampleCluster())
		api.PutService(SampleService())
		api.PutTaskDefinition(SampleTaskDefinition())
		t := SampleTask()
		t.Host = "local-1"
		api.ScheduleTask(t)
		api.GetService("test")
	}()

	time.Sleep(15 * time.Second)

	if c == 0 {
		t.Fatal("could not get any events from config monitor")
	}
}
