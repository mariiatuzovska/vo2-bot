package telegram

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	b.reply(msg, "👋 VO2 coaching bot\n\n"+
		"/login — connect your Strava account\n"+
		"/pull  — sync latest activities\n"+
		"/help  — show this message")
}

func (b *Bot) handleLogin(msg *tgbotapi.Message) {
	url := b.strava.AuthURL(msg.Chat.ID)
	b.reply(msg, "Connect your Strava account:\n"+url)
}

func (b *Bot) handlePull(ctx context.Context, msg *tgbotapi.Message) {
	b.reply(msg, "Syncing…")

	result, err := b.strava.Sync(ctx, msg.Chat.ID)
	if err != nil {
		b.reply(msg, "Error: "+err.Error())
		return
	}

	text := fmt.Sprintf("Pulled %d new activities (total: %d).", result.Added, result.Total)
	if result.Latest != nil {
		text += fmt.Sprintf("\nLatest: %s — %s",
			result.Latest.SportType,
			result.Latest.StartDate.Format("2 Jan 2006"))
	}
	b.reply(msg, text)
}
