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

	vo2bot "github.com/mariiatuzovska/vo2-bot"
	"github.com/mariiatuzovska/vo2-bot/internal/apple"
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

	if err := store.Migrate(cfg.DatabaseURL, vo2bot.MigrationsFS); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer db.Close()

	appleSvc := &apple.Service{
		Source: &apple.LocalSource{BaseDir: cfg.AppleArchiveDir},
		Store:  apple.NewStore(db.Pool),
	}
	appleHandler := &apple.Handler{Service: appleSvc}

	mux := http.NewServeMux()
	appleHandler.Register(mux)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("http listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	log.Println("vo2-bot started")
	<-ctx.Done()
	log.Println("vo2-bot shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}
