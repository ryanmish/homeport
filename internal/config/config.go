package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Server settings
	ListenAddr string `yaml:"listen_addr"`

	// Port scanning
	PortRangeMin int `yaml:"port_range_min"`
	PortRangeMax int `yaml:"port_range_max"`
	ScanInterval int `yaml:"scan_interval_seconds"`

	// Paths
	ReposDir string `yaml:"repos_dir"`
	DataDir  string `yaml:"data_dir"`
	UIDir    string `yaml:"ui_dir"`

	// External URLs (for generating shareable links)
	ExternalURL string `yaml:"external_url"`

	// Dev mode (uses lsof instead of /proc, different paths)
	DevMode bool `yaml:"-"`
}

func Default() *Config {
	return &Config{
		ListenAddr:   ":8080",
		PortRangeMin: 3000,
		PortRangeMax: 9999,
		ScanInterval: 5,
		ReposDir:     "/srv/homeport/repos",
		DataDir:      "/srv/homeport/data",
		UIDir:        "/srv/homeport/ui",
		ExternalURL:  "http://localhost:8080",
		DevMode:      false,
	}
}

func DefaultDev() *Config {
	cfg := Default()
	cfg.DevMode = true
	cfg.ReposDir = "./data/repos"
	cfg.DataDir = "./data"
	cfg.UIDir = "./ui/dist"
	return cfg
}

func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "homeport.db")
}

func (c *Config) EnsureDirs() error {
	if err := os.MkdirAll(c.ReposDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return err
	}
	return nil
}
