package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"sss/internal/api"
	"sss/internal/auth"
	"sss/internal/config"
	"sss/internal/storage"
	"sss/internal/utils"
)

func main() {
	// 命令行参数（运行时不可修改的配置）
	host := flag.String("host", "0.0.0.0", "监听地址")
	port := flag.Int("port", 8080, "监听端口")
	dbPath := flag.String("db", "./data/metadata.db", "数据库路径")
	dataPath := flag.String("data", "./data/buckets", "数据存储路径")
	logLevel := flag.String("log", "info", "日志级别 (debug/info/warn/error)")
	flag.Parse()

	// 1. 创建默认配置并应用命令行参数
	cfg := config.NewDefault()
	cfg.Server.Host = *host
	cfg.Server.Port = *port
	cfg.Storage.DBPath = *dbPath
	cfg.Storage.DataPath = *dataPath
	cfg.Log.Level = *logLevel

	// 初始化日志
	utils.InitLogger(cfg.Log.Level)
	utils.Info("SSS Server starting", "version", config.Version)

	// 2. 确保数据目录存在
	if err := os.MkdirAll(filepath.Dir(cfg.Storage.DBPath), 0755); err != nil {
		utils.Error("创建数据目录失败", "error", err)
		os.Exit(1)
	}

	// 3. 初始化元数据存储
	metadata, err := storage.NewMetadataStore(cfg.Storage.DBPath)
	if err != nil {
		utils.Error("初始化数据库失败", "error", err)
		os.Exit(1)
	}
	defer metadata.Close()

	// 4. 从数据库加载配置（如果已安装）
	config.LoadFromDB(metadata)

	// 4.1 初始化信任代理缓存
	utils.ReloadTrustedProxies(config.Global.Security.TrustedProxies)
	if config.Global.Security.TrustedProxies != "" {
		utils.Info("信任代理已配置", "cidrs", config.Global.Security.TrustedProxies)
	}

	// 4.2 初始化 GeoIP 服务（GeoIP.mmdb 存放在数据库同级目录）
	utils.InitGeoIP(config.Global.Storage.DBPath)

	// 4.3 初始化 GeoStats 服务
	storage.InitGeoStatsService(metadata)
	if config.Global.GeoStats.Enabled {
		utils.Info("GeoStats 已启用", "mode", config.Global.GeoStats.Mode)
	}

	// 5. 初始化文件存储（使用可能更新后的路径）
	filestore, err := storage.NewFileStore(config.Global.Storage.DataPath)
	if err != nil {
		utils.Error("初始化文件存储失败", "error", err)
		os.Exit(1)
	}

	// 6. 初始化 API Key 缓存
	auth.InitAPIKeyCache(metadata)
	utils.Info("API Key 缓存已初始化")

	// 7. 创建服务器
	server := api.NewServer(metadata, filestore)

	// 8. 显示启动信息
	addr := fmt.Sprintf("%s:%d", config.Global.Server.Host, config.Global.Server.Port)

	if metadata.IsInstalled() {
		utils.Info("系统已安装", "admin", config.Global.Auth.AdminUsername)
		if config.Global.Auth.AccessKeyID != "" {
			utils.Info("API Key 已配置", "id", config.Global.Auth.AccessKeyID)
		}
	} else {
		utils.Warn("系统尚未安装，请访问 Web 界面完成初始化")
	}

	// 9. 启动 HTTP 服务（带超时设置）
	// 使用 gzip 中间件包装 server，对文本资源进行压缩
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      utils.GzipHandler(server),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		utils.Info("服务器启动", "address", addr, "region", config.Global.Server.Region)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Error("服务器异常", "error", err)
			os.Exit(1)
		}
	}()

	// 10. 等待终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	utils.Info("收到终止信号，正在关闭服务器...", "signal", sig.String())

	// 11. 优雅关闭（等待最多 30 秒处理完当前请求）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		utils.Error("服务器关闭失败", "error", err)
		os.Exit(1)
	}

	// 停止 GeoStats 服务（刷新缓冲区）
	storage.GetGeoStatsService().Stop()

	utils.Info("服务器已安全关闭")
}
