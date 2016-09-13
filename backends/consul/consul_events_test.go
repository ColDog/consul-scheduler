package consul

import (
	"github.com/coldog/sked/api"

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
	RunConsulApiTest(func(a *ConsulApi) {
		a.Start()

		listener := make(chan string)
		a.Subscribe("test-health", "health::*", listener)
		defer a.UnSubscribe("test-health")

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
	})
}

func TestConfigEventWaiting(t *testing.T) {
	RunConsulApiTest(func(a *ConsulApi) {
		a.Start()

		listener := make(chan string, 30)
		a.Subscribe("test-config", "*", listener)
		defer a.UnSubscribe("test-config")

		c := 0
		go func() {
			for val := range listener {
				fmt.Printf("event: %s\n", val)
				c++
			}
		}()

		go func() {
			time.Sleep(1 * time.Second)
			fmt.Println("---> events sending")
			a.PutCluster(api.SampleCluster())
			a.PutService(api.SampleService())
			a.PutTaskDefinition(api.SampleTaskDefinition())
			t := api.SampleTask()
			t.Host = "local-1"
			t.Scheduled = true
			a.PutTask(t)
			a.GetService("test")
		}()

		time.Sleep(3 * time.Second)

		if c == 0 {
			t.Fatal("could not get any events from config monitor")
		}
	})
}
