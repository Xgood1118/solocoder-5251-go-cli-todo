package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ThemeConfig struct {
	P0       string `yaml:"p0"`
	P1       string `yaml:"p1"`
	P2       string `yaml:"p2"`
	P3       string `yaml:"p3"`
	Done     string `yaml:"done"`
	Category string `yaml:"category"`
	Deadline string `yaml:"deadline"`
}

type Config struct {
	StoragePath  string      `yaml:"storage_path"`
	ArchiveDays  int         `yaml:"archive_days"`
	Theme        ThemeConfig `yaml:"theme"`
	DedupMinutes int         `yaml:"dedup_minutes"`
}

var defaultConfig = Config{
	StoragePath:  "",
	ArchiveDays:  30,
	DedupMinutes: 5,
	Theme: ThemeConfig{
		P0:       "red",
		P1:       "yellow",
		P2:       "green",
		P3:       "white",
		Done:     "green",
		Category: "cyan",
		Deadline: "magenta",
	},
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gocli-todo"
	}
	return filepath.Join(home, ".gocli-todo")
}

func defaultTasksFile() string {
	return filepath.Join(defaultDataDir(), "tasks.json")
}

func defaultConfigFile() string {
	return filepath.Join(defaultDataDir(), "config.yaml")
}

func LoadConfig() (*Config, error) {
	cfg := defaultConfig
	cfg.StoragePath = defaultTasksFile()

	configFile := defaultConfigFile()
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	mergeConfig(&cfg, &loaded)

	if cfg.StoragePath == "" {
		cfg.StoragePath = defaultTasksFile()
	}

	return &cfg, nil
}

func mergeConfig(dst, src *Config) {
	if src.StoragePath != "" {
		dst.StoragePath = src.StoragePath
	}
	if src.ArchiveDays > 0 {
		dst.ArchiveDays = src.ArchiveDays
	}
	if src.DedupMinutes > 0 {
		dst.DedupMinutes = src.DedupMinutes
	}
	if src.Theme.P0 != "" {
		dst.Theme.P0 = src.Theme.P0
	}
	if src.Theme.P1 != "" {
		dst.Theme.P1 = src.Theme.P1
	}
	if src.Theme.P2 != "" {
		dst.Theme.P2 = src.Theme.P2
	}
	if src.Theme.P3 != "" {
		dst.Theme.P3 = src.Theme.P3
	}
	if src.Theme.Done != "" {
		dst.Theme.Done = src.Theme.Done
	}
	if src.Theme.Category != "" {
		dst.Theme.Category = src.Theme.Category
	}
	if src.Theme.Deadline != "" {
		dst.Theme.Deadline = src.Theme.Deadline
	}
}

func SaveConfig(cfg *Config) error {
	configFile := defaultConfigFile()
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var existing *Config
	if data, err := os.ReadFile(configFile); err == nil {
		var loaded Config
		if yaml.Unmarshal(data, &loaded) == nil {
			existing = &loaded
		}
	}

	toSave := *cfg
	if existing != nil {
		merged := *existing
		mergeConfig(&merged, cfg)
		toSave = merged
	}

	data, err := yaml.Marshal(&toSave)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpFile := configFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmpFile, configFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename config: %w", err)
	}

	return nil
}
