package sync

import (
	"context"
	"log"
	"time"

	"colosscious.com/github-sync/internal/config"
)

// Scheduler 定時排程器
type Scheduler struct {
	syncer    *Syncer
	interval  time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	reloadCh  <-chan struct{}
}

// NewScheduler 建立排程器
func NewScheduler(syncer *Syncer, interval time.Duration, reloadCh <-chan struct{}) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		syncer:   syncer,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		reloadCh: reloadCh,
	}
}

// Start 啟動排程器
func (s *Scheduler) Start() {
	log.Printf("Scheduler started with interval: %s", s.interval)

	// 立即執行一次
	if err := s.syncer.Run(); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定時執行
			if err := s.syncer.Run(); err != nil {
				log.Printf("Sync failed: %v", err)
			}

		case <-s.reloadCh:
			// 配置已重新載入
			log.Println("Config reloaded, updating scheduler...")

			// 更新 syncer 配置
			cfg := config.GetConfig()
			s.syncer.UpdateConfig(cfg)

			// 更新 interval
			newInterval, err := cfg.GetSyncInterval()
			if err != nil {
				log.Printf("Invalid interval in new config: %v", err)
				continue
			}

			if newInterval != s.interval {
				s.interval = newInterval
				ticker.Reset(newInterval)
				log.Printf("Scheduler interval updated to: %s", newInterval)
			}

		case <-s.ctx.Done():
			// 收到停止訊號
			log.Println("Scheduler stopped")
			return
		}
	}
}

// Stop 停止排程器
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.cancel()
}
