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
	"github.com/mariiatuzovska/vo2-bot/internal/strava"
	"github.com/mariiatuzovska/vo2-bot/internal/telegram"
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

	mux := http.NewServeMux()
	registerApple(mux, cfg, db)
	registerTelegram(ctx, cfg, registerStrava(mux, cfg, db))

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

func registerApple(mux *http.ServeMux, cfg *config.Config, db *store.Store) {
	svc := &apple.Service{
		Source: &apple.LocalSource{BaseDir: cfg.AppleArchiveDir},
		Store:  apple.NewStore(db.Pool),
	}
	(&apple.Handler{Service: svc}).Register(mux)
}

func registerStrava(mux *http.ServeMux, cfg *config.Config, db *store.Store) *strava.Client {
	client := strava.New(
		cfg.StravaClientID,
		cfg.StravaClientSecret,
		cfg.StravaRedirectURL,
		db.Pool,
	)
	(&strava.Handler{Client: client}).Register(mux)
	return client
}

func registerTelegram(ctx context.Context, cfg *config.Config, stravaClient *strava.Client) {
	if cfg.TelegramBotToken == "" {
		log.Println("TELEGRAM_BOT_TOKEN not set — bot disabled")
		return
	}
	bot, err := telegram.New(cfg.TelegramBotToken, cfg.TelegramAllowedChatIDs, stravaClient)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	go bot.Run(ctx)
}
