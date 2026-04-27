package telegram

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/mariiatuzovska/vo2-bot/internal/apple"
)

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	b.reply(msg, "👋 VO2 coaching bot\n\n"+
		"/strava — sync latest Strava activities\n"+
		"/apple  — import latest local Apple Health archive\n"+
		"/help   — show this message")
}

func (b *Bot) handleSyncStrava(ctx context.Context, msg *tgbotapi.Message) {
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

func (b *Bot) handleApple(ctx context.Context, msg *tgbotapi.Message) {
	b.reply(msg, "Importing latest local Apple Health archive…")

	result, err := b.apple.Import(ctx, apple.ImportRequest{Source: "local"})
	if err != nil {
		b.reply(msg, "Error: "+err.Error())
		return
	}

	text := fmt.Sprintf("Imported %d workouts, %d metrics.", result.WorkoutsAdded, result.MetricsAdded)
	if result.RangeStart != nil && result.RangeEnd != nil {
		text += fmt.Sprintf("\nRange: %s → %s",
			result.RangeStart.Format("2 Jan 2006"),
			result.RangeEnd.Format("2 Jan 2006"))
	}
	if result.Latest != nil {
		text += fmt.Sprintf("\nLatest: %s — %s",
			result.Latest.Name,
			result.Latest.StartDate.Format("2 Jan 2006"))
	}
	b.reply(msg, text)
}
