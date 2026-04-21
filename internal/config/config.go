package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL            string
	StravaClientID         string
	StravaClientSecret     string
	StravaRedirectURL      string
	TelegramBotToken       string
	TelegramAllowedChatIDs string
	AnthropicAPIKey        string
	ClaudeModel            string
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("CLAUDE_MODEL", "claude-opus-4-7")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{
		DatabaseURL:            v.GetString("DATABASE_URL"),
		StravaClientID:         v.GetString("STRAVA_CLIENT_ID"),
		StravaClientSecret:     v.GetString("STRAVA_CLIENT_SECRET"),
		StravaRedirectURL:      v.GetString("STRAVA_REDIRECT_URL"),
		TelegramBotToken:       v.GetString("TELEGRAM_BOT_TOKEN"),
		TelegramAllowedChatIDs: v.GetString("TELEGRAM_ALLOWED_CHAT_IDS"),
		AnthropicAPIKey:        v.GetString("ANTHROPIC_API_KEY"),
		ClaudeModel:            v.GetString("CLAUDE_MODEL"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}
