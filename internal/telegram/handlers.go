package telegram

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/mariiatuzovska/vo2-bot/internal/apple"
	"github.com/mariiatuzovska/vo2-bot/internal/claude"
)

const coachSystemPromptHeader = `You are an endurance-sports coaching assistant for a single athlete.
You have been given a snapshot of the athlete's recent training data (Strava activities and, when available, Apple Health daily metrics: HRV, resting HR, sleep) below. Use it to ground your answers across the whole conversation.
Be concise. Cite specific sessions by date when relevant. Flag overtraining, monotony, or recovery red flags if you see them. If data is insufficient to answer, say so.

`

const telegramMaxMessage = 4000

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	b.reply(msg, "👋 VO2 coaching bot\n\n"+
		"/strava — sync latest Strava activities\n"+
		"/apple  — import latest local Apple Health archive\n"+
		"/coach — start a coaching chat (loads recent metrics into context)\n"+
		"   then send plain messages to talk with the coach\n"+
		"/end — end the current coaching chat\n"+
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

func (b *Bot) handleCoach(ctx context.Context, msg *tgbotapi.Message) {
	athleteID, err := b.coach.ResolveAthlete(ctx, msg.Chat.ID)
	if err != nil {
		b.reply(msg, "No Strava account linked to this chat. Run /strava first.")
		return
	}

	b.reply(msg, "Loading metrics…")

	session := &coachSession{loading: true}
	b.mu.Lock()
	b.sessions[msg.Chat.ID] = session
	b.mu.Unlock()

	contextBlock, err := b.coach.Build(ctx, athleteID, 14)
	if err != nil {
		b.mu.Lock()
		delete(b.sessions, msg.Chat.ID)
		b.mu.Unlock()
		b.reply(msg, "Error building context: "+err.Error())
		return
	}

	b.mu.Lock()
	session.system = coachSystemPromptHeader + contextBlock
	session.loading = false
	b.mu.Unlock()

	b.reply(msg, "Coach session started. Send a message to begin. /end to finish.")
}

func (b *Bot) handleCoachFollowup(ctx context.Context, msg *tgbotapi.Message) {
	b.mu.Lock()
	session, ok := b.sessions[msg.Chat.ID]
	loading := ok && session.loading
	b.mu.Unlock()
	if !ok {
		return
	}
	if loading {
		b.reply(msg, "Still loading metrics — please wait a moment.")
		return
	}

	b.reply(msg, "Thinking…")

	session.history = append(session.history, claude.Message{Role: "user", Content: msg.Text})

	answer, err := b.claude.Chat(ctx, session.system, session.history)
	if err != nil {
		b.reply(msg, "Claude error: "+err.Error())
		// roll back the unanswered user turn so the next attempt isn't malformed
		session.history = session.history[:len(session.history)-1]
		return
	}
	session.history = append(session.history, claude.Message{Role: "assistant", Content: answer})

	for _, chunk := range splitForTelegram(answer, telegramMaxMessage) {
		b.reply(msg, chunk)
	}
}

func (b *Bot) handleEndCoach(msg *tgbotapi.Message) {
	b.mu.Lock()
	_, ok := b.sessions[msg.Chat.ID]
	delete(b.sessions, msg.Chat.ID)
	b.mu.Unlock()
	if !ok {
		b.reply(msg, "No active coach session.")
		return
	}
	b.reply(msg, "Coach session ended. Run /coach to start a new one.")
}

// splitForTelegram breaks long replies on paragraph/line boundaries so each
// chunk stays under Telegram's 4096-char message limit.
func splitForTelegram(s string, max int) []string {
	if len(s) <= max {
		return []string{s}
	}
	var out []string
	for len(s) > max {
		cut := strings.LastIndex(s[:max], "\n")
		if cut <= 0 {
			cut = max
		}
		out = append(out, s[:cut])
		s = strings.TrimLeft(s[cut:], "\n")
	}
	if len(s) > 0 {
		out = append(out, s)
	}
	return out
}
