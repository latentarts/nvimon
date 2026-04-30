package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultRefreshInterval = time.Second
	DefaultHistoryLength   = 120
	DefaultConnectTimeout  = 2 * time.Second
	DefaultRequestTimeout  = 2 * time.Second
	DefaultAgentBind       = "127.0.0.1:9910"
)

type Config struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	HistoryLength   int           `yaml:"history_length"`
	Timeouts        Timeouts      `yaml:"timeouts"`
	Agent           AgentConfig   `yaml:"agent"`
	Hosts           []HostConfig  `yaml:"hosts"`
}

type Timeouts struct {
	Connect time.Duration `yaml:"connect"`
	Request time.Duration `yaml:"request"`
}

type AgentConfig struct {
	BindAddress string `yaml:"bind_address"`
	AuthToken   string `yaml:"auth_token,omitempty"`
}

type HostMode string

const (
	HostModeLocal  HostMode = "local"
	HostModeRemote HostMode = "remote"
)

type HostConfig struct {
	Name  string   `yaml:"name"`
	Mode  HostMode `yaml:"mode"`
	URL   string   `yaml:"url,omitempty"`
	Token string   `yaml:"token,omitempty"`
}

func Default() Config {
	return Config{
		RefreshInterval: DefaultRefreshInterval,
		HistoryLength:   DefaultHistoryLength,
		Timeouts: Timeouts{
			Connect: DefaultConnectTimeout,
			Request: DefaultRequestTimeout,
		},
		Agent: AgentConfig{
			BindAddress: DefaultAgentBind,
		},
		Hosts: []HostConfig{
			{
				Name: "localhost",
				Mode: HostModeLocal,
			},
		},
	}
}

func DefaultPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}

	return filepath.Join(configHome, "nvimon", "config.yaml")
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultPath()
	}
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	applyDefaults(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = DefaultRefreshInterval
	}
	if cfg.HistoryLength <= 0 {
		cfg.HistoryLength = DefaultHistoryLength
	}
	if cfg.Timeouts.Connect <= 0 {
		cfg.Timeouts.Connect = DefaultConnectTimeout
	}
	if cfg.Timeouts.Request <= 0 {
		cfg.Timeouts.Request = DefaultRequestTimeout
	}
	if strings.TrimSpace(cfg.Agent.BindAddress) == "" {
		cfg.Agent.BindAddress = DefaultAgentBind
	}
	if len(cfg.Hosts) == 0 {
		cfg.Hosts = Default().Hosts
	}

	for i := range cfg.Hosts {
		host := &cfg.Hosts[i]
		if strings.TrimSpace(host.Name) == "" {
			if host.Mode == HostModeRemote && host.URL != "" {
				host.Name = host.URL
			} else {
				host.Name = fmt.Sprintf("host-%d", i+1)
			}
		}
		if host.Mode == "" {
			host.Mode = HostModeLocal
		}
	}
}

func validate(cfg Config) error {
	for _, host := range cfg.Hosts {
		switch host.Mode {
		case HostModeLocal:
		case HostModeRemote:
			if strings.TrimSpace(host.URL) == "" {
				return fmt.Errorf("remote host %q missing url", host.Name)
			}
		default:
			return fmt.Errorf("host %q has invalid mode %q", host.Name, host.Mode)
		}
	}
	return nil
}
