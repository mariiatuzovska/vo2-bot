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
