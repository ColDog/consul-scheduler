package config

import (
	"github.com/hashicorp/consul/api"
)

type Backend string

const (
	CONSUL Backend = "consul"
	ETCD Backend   = "etcd"
)

func NewConfig() *Config {
	return &Config{
		Addr: "0.0.0.0",
		Port: 8231,
		Advertise: "127.0.0.1:8231",
		LogLevel: "info",
		Backend: CONSUL,
		ConsulConfig: api.DefaultConfig(),
	}
}

type Config struct {
	Addr         string       `json:"addr"`
	Port         int          `json:"port"`
	Advertise    string       `json:"advertise"`
	LogLevel     string       `json:"log_level"`
	Version      string       `json:"version"`
	Backend      Backend      `json:"backend"`
	ConsulConfig *api.Config  `json:"consul_config"`
}
