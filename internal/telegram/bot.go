package telegram

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/mariiatuzovska/vo2-bot/internal/strava"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	strava  *strava.Client
	allowed map[int64]bool // empty = allow all (dev mode)
}

func New(token string, allowedIDs string, stravaClient *strava.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	log.Printf("telegram: authorized as @%s", api.Self.UserName)

	allowed := parseAllowedIDs(allowedIDs)
	if len(allowed) == 0 {
		log.Println("telegram: TELEGRAM_ALLOWED_CHAT_IDS not set — accepting all chats (dev mode)")
	}

	return &Bot{api: api, strava: stravaClient, allowed: allowed}, nil
}

// Run starts the long-poll loop and blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil {
				continue
			}
			msg := update.Message
			log.Printf("telegram: chat_id=%d @%s: %q", msg.Chat.ID, msg.Chat.UserName, msg.Text)
			go b.dispatch(ctx, msg)
		}
	}
}

func (b *Bot) dispatch(ctx context.Context, msg *tgbotapi.Message) {
	if !b.isAllowed(msg.Chat.ID) {
		b.reply(msg, "Access denied.")
		return
	}
	if !msg.IsCommand() {
		return
	}
	switch msg.Command() {
	case "start", "help":
		b.handleStart(msg)
	case "login":
		b.handleLogin(msg)
	case "pull":
		b.handlePull(ctx, msg)
	default:
		b.reply(msg, "Unknown command. Use /help to see available commands.")
	}
}

func (b *Bot) reply(msg *tgbotapi.Message, text string) {
	out := tgbotapi.NewMessage(msg.Chat.ID, text)
	out.ReplyToMessageID = msg.MessageID
	if _, err := b.api.Send(out); err != nil {
		log.Printf("telegram: send error: %v", err)
	}
}

func (b *Bot) isAllowed(chatID int64) bool {
	if len(b.allowed) == 0 {
		return true
	}
	return b.allowed[chatID]
}

func parseAllowedIDs(raw string) map[int64]bool {
	m := make(map[int64]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err == nil && id != 0 {
			m[id] = true
		}
	}
	return m
}
