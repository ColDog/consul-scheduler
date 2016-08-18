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

	go api.monitorHealth()

	listener := make(chan string)
	api.Listen("health::*", listener)

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

	go api.monitorConfig()

	listener := make(chan string)
	api.Listen("config::*", listener)

	c := 0
	go func() {
		for val := range listener {
			fmt.Printf("event: %s\n", val)
			c++
		}
	}()

	time.Sleep(15 * time.Second)

	if c == 0 {
		t.Fatal("could not get any events from config monitor")
	}
}
