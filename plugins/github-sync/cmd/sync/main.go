package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"colosscious.com/github-sync/internal/config"
	"colosscious.com/github-sync/internal/storage"
	"colosscious.com/github-sync/internal/sync"
)

func main() {
	// 解析命令列參數
	configPath := flag.String("config", "", "Path to config file (default: $CONFIG_PATH or ./config.yaml)")
	flag.Parse()

	// 載入配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 設定 log 輸出
	if cfg.Sync.OnError.LogFile != "" {
		logFile, err := os.OpenFile(cfg.Sync.OnError.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("Warning: Failed to open log file: %v", err)
		} else {
			defer logFile.Close()
			log.SetOutput(logFile)
		}
	}

	log.Println("=== Redmine-GitHub Sync Service ===")
	log.Printf("Redmine URL: %s", cfg.Redmine.URL)
	log.Printf("Sync interval: %s", cfg.Sync.Interval)
	log.Printf("Projects: %d", len(cfg.Redmine.Projects))

	// 連接資料庫
	db, err := storage.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// 建立同步器
	syncer := sync.NewSyncer(cfg, db)

	// 取得同步間隔
	interval, err := cfg.GetSyncInterval()
	if err != nil {
		log.Fatalf("Invalid sync interval: %v", err)
	}

	// 建立排程器
	scheduler := sync.NewScheduler(syncer, interval, config.GetReloadChannel())

	// 處理優雅關閉
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 啟動排程器（在背景執行）
	go scheduler.Start()

	// 等待終止訊號
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)

	// 停止排程器
	scheduler.Stop()

	log.Println("Service stopped gracefully")
}
