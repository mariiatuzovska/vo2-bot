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

-- NULL or empty arrays disable the corresponding filter.
-- cardinality(NULL) returns NULL in PG, so we wrap with COALESCE.

-- name: ListWorkouts :many
SELECT id, name, source, is_indoor, location,
       started_at, ended_at, duration_seconds,
       distance_km, active_energy_kj,
       avg_hr_bpm, max_hr_bpm, min_hr_bpm,
       elevation_up_m, avg_speed, speed_units,
       step_cadence, humidity_pct,
       temperature, temperature_units,
       intensity, payload, created_at
  FROM apple_workouts
 WHERE started_at >= @from_at
   AND started_at <  @to_at
   AND (COALESCE(cardinality(@names::text[]), 0) = 0 OR name = ANY(@names::text[]))
 ORDER BY started_at DESC
 LIMIT @lim;

-- name: ListWorkoutHeartRate :many
SELECT workout_id, measured_at, min_bpm, max_bpm, avg_bpm
  FROM apple_workout_heart_rate
 WHERE workout_id = ANY(@workout_ids::uuid[])
 ORDER BY workout_id, measured_at;

-- name: ListWorkoutRoute :many
SELECT id, workout_id, recorded_at, latitude, longitude,
       altitude_m, speed, horizontal_accuracy, course_accuracy
  FROM apple_workout_route
 WHERE workout_id = ANY(@workout_ids::uuid[])
 ORDER BY workout_id, recorded_at;

-- name: ListDailyMetrics :many
SELECT metric_name, measured_at, source, qty, units
  FROM apple_daily_metrics
 WHERE measured_at >= @from_at
   AND measured_at <  @to_at
   AND (COALESCE(cardinality(@names::text[]),   0) = 0 OR metric_name = ANY(@names::text[]))
   AND (COALESCE(cardinality(@sources::text[]), 0) = 0 OR source      = ANY(@sources::text[]))
 ORDER BY metric_name, measured_at;

-- name: ListWorkoutNames :many
SELECT DISTINCT name FROM apple_workouts ORDER BY name;

-- name: ListMetricNames :many
SELECT DISTINCT metric_name FROM apple_daily_metrics ORDER BY metric_name;
