package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Auth    AuthConfig    `yaml:"auth"`
	Log     LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Region string `yaml:"region"`
}

type StorageConfig struct {
	DataPath string `yaml:"data_path"`
	DBPath   string `yaml:"db_path"`
}

type AuthConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

var Global *Config

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 9000
	}
	if cfg.Server.Region == "" {
		cfg.Server.Region = "us-east-1"
	}
	if cfg.Storage.DataPath == "" {
		cfg.Storage.DataPath = "./data/buckets"
	}
	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = "./data/metadata.db"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	Global = cfg
	return cfg, nil
}
