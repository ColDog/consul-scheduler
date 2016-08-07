#!/usr/bin/env bash

env GOOS=linux GOARCH=amd64 go build -o consul-scheduler github.com/coldog/scheduler
