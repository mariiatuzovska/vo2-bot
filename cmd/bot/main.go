package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mariiatuzovska/vo2-bot/internal/config"
	"github.com/mariiatuzovska/vo2-bot/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	log.Println("vo2-bot started")
	<-ctx.Done()
	log.Println("vo2-bot shutting down")
}
