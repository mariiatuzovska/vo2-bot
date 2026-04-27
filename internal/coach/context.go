package coach

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mariiatuzovska/vo2-bot/internal/store/queries"
)

const (
	defaultDays      = 14
	activityLimit    = 50
	dailyMetricLimit = 200
)

type Builder struct {
	q *queries.Queries
}

func NewBuilder(pool *pgxpool.Pool) *Builder {
	return &Builder{q: queries.New(pool)}
}

// ResolveAthlete returns the Strava athlete ID linked to a Telegram chat.
func (b *Builder) ResolveAthlete(ctx context.Context, chatID int64) (int64, error) {
	return b.q.ResolveAthleteByChat(ctx, chatID)
}

// Build returns a compact text block summarising the athlete's last `days` of
// Strava activities and Apple daily metrics, suitable for prompting Claude.
func (b *Builder) Build(ctx context.Context, athleteID int64, days int) (string, error) {
	if days <= 0 {
		days = defaultDays
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	activities, err := b.q.ListRecentStravaActivities(ctx, queries.ListRecentStravaActivitiesParams{
		AthleteID: athleteID,
		Since:     since,
		Lim:       activityLimit,
	})
	if err != nil {
		return "", fmt.Errorf("list activities: %w", err)
	}

	metrics, err := b.q.ListDailyMetrics(ctx, queries.ListDailyMetricsParams{
		FromAt: since,
		ToAt:   time.Now().Add(24 * time.Hour),
		Names:  []string{"heart_rate_variability", "resting_heart_rate", "sleep_analysis"},
	})
	if err != nil {
		return "", fmt.Errorf("list metrics: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Training data for the last %d days (today: %s).\n\n",
		days, time.Now().Format("2006-01-02"))

	sb.WriteString("Strava activities (newest first):\n")
	if len(activities) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, a := range activities {
			sb.WriteString("  - ")
			sb.WriteString(formatActivity(a))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\nApple daily metrics:\n")
	if len(metrics) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, m := range metrics {
			fmt.Fprintf(&sb, "  - %s: %s = %.1f %s\n",
				m.MeasuredAt.Format("2006-01-02"),
				m.MetricName, m.Qty, m.Units)
		}
	}

	return sb.String(), nil
}

func formatActivity(a queries.StravaActivity) string {
	parts := []string{
		a.StartAt.Format("2006-01-02"),
		a.SportType,
	}
	if a.DistanceM != nil && *a.DistanceM > 0 {
		parts = append(parts, fmt.Sprintf("%.1f km", *a.DistanceM/1000))
	}
	if a.MovingTimeS != nil && *a.MovingTimeS > 0 {
		parts = append(parts, fmt.Sprintf("%d min", *a.MovingTimeS/60))
	}
	if a.ElevationGainM != nil && *a.ElevationGainM > 0 {
		parts = append(parts, fmt.Sprintf("+%.0fm", *a.ElevationGainM))
	}
	if a.AverageHeartrate != nil && *a.AverageHeartrate > 0 {
		hr := fmt.Sprintf("avgHR %.0f", *a.AverageHeartrate)
		if a.MaxHeartrate != nil && *a.MaxHeartrate > 0 {
			hr += fmt.Sprintf("/maxHR %.0f", *a.MaxHeartrate)
		}
		parts = append(parts, hr)
	}
	if a.AverageWatts != nil && *a.AverageWatts > 0 {
		parts = append(parts, fmt.Sprintf("avgW %.0f", *a.AverageWatts))
	}
	if a.SufferScore != nil && *a.SufferScore > 0 {
		parts = append(parts, fmt.Sprintf("suffer %d", *a.SufferScore))
	}
	if a.Trainer != nil && *a.Trainer {
		parts = append(parts, "indoor")
	}
	if a.Name != "" {
		parts = append(parts, fmt.Sprintf("%q", a.Name))
	}
	return strings.Join(parts, " · ")
}
