package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"sss/internal/api"
	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	utils.InitLogger(cfg.Log.Level)
	utils.Info("SSS Server starting", "version", "1.0.0")

	// 确保数据目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.Storage.DBPath), 0755); err != nil {
		utils.Error("创建数据目录失败", "error", err)
		os.Exit(1)
	}

	// 初始化元数据存储
	metadata, err := storage.NewMetadataStore(cfg.Storage.DBPath)
	if err != nil {
		utils.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer metadata.Close()

	// 初始化文件存储
	filestore, err := storage.NewFileStore(cfg.Storage.DataPath)
	if err != nil {
		utils.Error("初始化文件存储失败", "error", err)
		os.Exit(1)
	}

	// 创建服务器
	server := api.NewServer(metadata, filestore)

	// 启动 HTTP 服务
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	utils.Info("服务器启动", "address", addr, "region", cfg.Server.Region)
	utils.Info("Access Key", "id", cfg.Auth.AccessKeyID)

	if err := http.ListenAndServe(addr, server); err != nil {
		utils.Error("服务器启动失败", "error", err)
		os.Exit(1)
	}
}
