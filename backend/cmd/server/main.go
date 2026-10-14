package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stpt/quiz/internal/config"
	"stpt/quiz/internal/httpapi"
	"stpt/quiz/internal/publish"
	"stpt/quiz/internal/quiz"
	"stpt/quiz/internal/rank"
	"stpt/quiz/internal/store"
	"stpt/quiz/internal/ws"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := openStoreWithRetry(ctx, cfg.DatabaseURL, cfg.PGPoolMax)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	rk := rank.New(cfg.RedisAddr)
	if err := rk.Ping(ctx); err != nil {
		log.Fatalf("redis: %v", err)
	}
	pub := publish.New(cfg.RedisAddr)

	if err := st.EnsureFooterDefaults(ctx); err != nil {
		log.Fatalf("footer defaults: %v", err)
	}

	svc := quiz.New(st, rk, pub, cfg.Title)
	svc.StartLeaderboardRefresh(ctx)
	hub := ws.NewHub(svc, pub.Subscribe(ctx)) // 排行榜改走 DB 全量排名
	go hub.Run(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(cfg, svc, hub).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("quiz server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// openStoreWithRetry 等待 compose 里 postgres 就绪（最多 30s）。
func openStoreWithRetry(ctx context.Context, dsn string, maxConns int) (*store.Store, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		st, err := store.Open(ctx, dsn, maxConns)
		if err == nil {
			return st, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, lastErr
}
