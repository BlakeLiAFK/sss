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
	DataPath        string `yaml:"data_path"`
	DBPath          string `yaml:"db_path"`
	MaxObjectSize   int64  `yaml:"max_object_size"`   // 全局最大对象大小（字节），0表示无限制
	MaxUploadSize   int64  `yaml:"max_upload_size"`   // 预签名URL最大上传大小（字节），0表示无限制
}

type AuthConfig struct {
	// 管理员账号 (Web UI 登录)
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`

	// 兼容旧配置：如果没有配置管理员账号，使用这个作为默认 API Key
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
	if cfg.Storage.MaxObjectSize == 0 {
		cfg.Storage.MaxObjectSize = 5 * 1024 * 1024 * 1024 // 默认5GB
	}
	if cfg.Storage.MaxUploadSize == 0 {
		cfg.Storage.MaxUploadSize = 5 * 1024 * 1024 * 1024 // 默认5GB
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}

	// 管理员账号默认值
	if cfg.Auth.AdminUsername == "" {
		cfg.Auth.AdminUsername = "admin"
	}
	if cfg.Auth.AdminPassword == "" {
		cfg.Auth.AdminPassword = "admin"
	}

	Global = cfg
	return cfg, nil
}
