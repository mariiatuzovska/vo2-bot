-- name: GetImportBySha256 :one
SELECT id, source_filename, source_sha256, range_start, range_end,
       workouts_added, metrics_added, imported_at
  FROM apple_imports
 WHERE source_sha256 = $1;

-- name: InsertImport :one
INSERT INTO apple_imports (
    source_filename, source_sha256, range_start, range_end,
    workouts_added, metrics_added
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, source_filename, source_sha256, range_start, range_end,
          workouts_added, metrics_added, imported_at;

-- name: UpsertWorkout :exec
INSERT INTO apple_workouts (
    id, name, source, is_indoor, location,
    started_at, ended_at, duration_seconds,
    distance_km, active_energy_kj,
    avg_hr_bpm, max_hr_bpm, min_hr_bpm,
    elevation_up_m, avg_speed, speed_units,
    step_cadence, humidity_pct,
    temperature, temperature_units,
    intensity, payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19, $20, $21, $22
)
ON CONFLICT (id) DO UPDATE SET
    name              = EXCLUDED.name,
    source            = EXCLUDED.source,
    is_indoor         = EXCLUDED.is_indoor,
    location          = EXCLUDED.location,
    started_at        = EXCLUDED.started_at,
    ended_at          = EXCLUDED.ended_at,
    duration_seconds  = EXCLUDED.duration_seconds,
    distance_km       = EXCLUDED.distance_km,
    active_energy_kj  = EXCLUDED.active_energy_kj,
    avg_hr_bpm        = EXCLUDED.avg_hr_bpm,
    max_hr_bpm        = EXCLUDED.max_hr_bpm,
    min_hr_bpm        = EXCLUDED.min_hr_bpm,
    elevation_up_m    = EXCLUDED.elevation_up_m,
    avg_speed         = EXCLUDED.avg_speed,
    speed_units       = EXCLUDED.speed_units,
    step_cadence      = EXCLUDED.step_cadence,
    humidity_pct      = EXCLUDED.humidity_pct,
    temperature       = EXCLUDED.temperature,
    temperature_units = EXCLUDED.temperature_units,
    intensity         = EXCLUDED.intensity,
    payload           = EXCLUDED.payload;

-- name: DeleteWorkoutHeartRate :exec
DELETE FROM apple_workout_heart_rate WHERE workout_id = $1;

-- name: DeleteWorkoutRoute :exec
DELETE FROM apple_workout_route WHERE workout_id = $1;

-- name: UpsertDailyMetric :exec
INSERT INTO apple_daily_metrics (metric_name, measured_at, source, qty, units)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (metric_name, measured_at, source) DO UPDATE SET
    qty   = EXCLUDED.qty,
    units = EXCLUDED.units;

-- ============================================================
-- Strava
-- ============================================================

-- name: UpsertStravaAthlete :exec
INSERT INTO strava_athletes (
    strava_athlete_id, username, firstname, lastname,
    city, country, sex, weight_kg, ftp_watts, profile_url, fetched_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    username    = EXCLUDED.username,
    firstname   = EXCLUDED.firstname,
    lastname    = EXCLUDED.lastname,
    city        = EXCLUDED.city,
    country     = EXCLUDED.country,
    sex         = EXCLUDED.sex,
    weight_kg   = EXCLUDED.weight_kg,
    ftp_watts   = EXCLUDED.ftp_watts,
    profile_url = EXCLUDED.profile_url,
    fetched_at  = now();

-- name: UpsertStravaTokens :exec
INSERT INTO strava_tokens (strava_athlete_id, access_token, refresh_token, expires_at, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    access_token  = EXCLUDED.access_token,
    refresh_token = EXCLUDED.refresh_token,
    expires_at    = EXCLUDED.expires_at,
    updated_at    = now();

-- name: GetStravaTokens :one
SELECT strava_athlete_id, access_token, refresh_token, expires_at, updated_at
  FROM strava_tokens
 WHERE strava_athlete_id = $1;

-- name: DeleteStravaTokens :exec
DELETE FROM strava_tokens WHERE strava_athlete_id = $1;

-- name: LinkTelegramChat :exec
INSERT INTO telegram_strava_links (telegram_chat_id, strava_athlete_id)
VALUES ($1, $2)
ON CONFLICT (telegram_chat_id) DO UPDATE SET
    strava_athlete_id = EXCLUDED.strava_athlete_id,
    linked_at         = now();

-- name: ResolveAthleteByChat :one
SELECT strava_athlete_id FROM telegram_strava_links WHERE telegram_chat_id = $1;

-- name: UpsertStravaActivity :exec
INSERT INTO strava_activities (
    strava_activity_id, strava_athlete_id, name, sport_type, workout_type,
    start_at, start_at_local, timezone,
    distance_m, moving_time_s, elapsed_time_s, elevation_gain_m,
    average_speed_mps, max_speed_mps, average_heartrate, max_heartrate,
    average_watts, average_cadence, suffer_score,
    trainer, commute, payload, fetched_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, now()
)
ON CONFLICT (strava_activity_id) DO UPDATE SET
    name              = EXCLUDED.name,
    sport_type        = EXCLUDED.sport_type,
    workout_type      = EXCLUDED.workout_type,
    start_at          = EXCLUDED.start_at,
    start_at_local    = EXCLUDED.start_at_local,
    timezone          = EXCLUDED.timezone,
    distance_m        = EXCLUDED.distance_m,
    moving_time_s     = EXCLUDED.moving_time_s,
    elapsed_time_s    = EXCLUDED.elapsed_time_s,
    elevation_gain_m  = EXCLUDED.elevation_gain_m,
    average_speed_mps = EXCLUDED.average_speed_mps,
    max_speed_mps     = EXCLUDED.max_speed_mps,
    average_heartrate = EXCLUDED.average_heartrate,
    max_heartrate     = EXCLUDED.max_heartrate,
    average_watts     = EXCLUDED.average_watts,
    average_cadence   = EXCLUDED.average_cadence,
    suffer_score      = EXCLUDED.suffer_score,
    trainer           = EXCLUDED.trainer,
    commute           = EXCLUDED.commute,
    payload           = EXCLUDED.payload,
    fetched_at        = now();

-- name: GetSyncState :one
SELECT strava_athlete_id, last_synced_at, last_activity_at
  FROM strava_sync_state
 WHERE strava_athlete_id = $1;

-- name: UpsertSyncState :exec
INSERT INTO strava_sync_state (strava_athlete_id, last_synced_at, last_activity_at)
VALUES ($1, now(), $2)
ON CONFLICT (strava_athlete_id) DO UPDATE SET
    last_synced_at   = now(),
    last_activity_at = EXCLUDED.last_activity_at;

-- name: GetRateLimit :one
SELECT short_limit, short_usage, daily_limit, daily_usage, updated_at
  FROM strava_rate_limit
 WHERE id = 1;

-- name: UpdateRateLimit :exec
UPDATE strava_rate_limit
   SET short_limit  = $1,
       short_usage  = $2,
       daily_limit  = $3,
       daily_usage  = $4,
       updated_at   = now()
 WHERE id = 1;

-- name: UnlinkTelegramChat :exec
DELETE FROM telegram_strava_links WHERE telegram_chat_id = $1;

-- name: CountStravaActivities :one
SELECT COUNT(*) FROM strava_activities WHERE strava_athlete_id = $1;
