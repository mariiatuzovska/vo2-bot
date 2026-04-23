-- Batch 6 — Apple Health integration
-- Tables populated by the archive importer; see PLAN.md for the ingest flow.

CREATE TABLE apple_workouts (
    id                  UUID PRIMARY KEY,
    name                TEXT             NOT NULL,
    source              TEXT,
    is_indoor           BOOLEAN,
    location            TEXT,
    started_at          TIMESTAMPTZ      NOT NULL,
    ended_at            TIMESTAMPTZ      NOT NULL,
    duration_seconds    DOUBLE PRECISION NOT NULL,
    distance_km         DOUBLE PRECISION,
    active_energy_kj    DOUBLE PRECISION,
    avg_hr_bpm          DOUBLE PRECISION,
    max_hr_bpm          DOUBLE PRECISION,
    min_hr_bpm          DOUBLE PRECISION,
    elevation_up_m      DOUBLE PRECISION,
    avg_speed           DOUBLE PRECISION,
    speed_units         TEXT,
    step_cadence        DOUBLE PRECISION,
    humidity_pct        DOUBLE PRECISION,
    temperature         DOUBLE PRECISION,
    temperature_units   TEXT,
    intensity           DOUBLE PRECISION,
    payload             JSONB            NOT NULL,
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT now()
);
CREATE INDEX apple_workouts_started_at_idx ON apple_workouts (started_at DESC);

CREATE TABLE apple_workout_heart_rate (
    workout_id  UUID             NOT NULL REFERENCES apple_workouts(id) ON DELETE CASCADE,
    measured_at TIMESTAMPTZ      NOT NULL,
    min_bpm     DOUBLE PRECISION,
    max_bpm     DOUBLE PRECISION,
    avg_bpm     DOUBLE PRECISION,
    PRIMARY KEY (workout_id, measured_at)
);

-- Surrogate PK because HAE emits multiple sub-second GPS points but stores
-- timestamps at second precision, so (workout_id, recorded_at) isn't unique.
CREATE TABLE apple_workout_route (
    id                  BIGSERIAL        PRIMARY KEY,
    workout_id          UUID             NOT NULL REFERENCES apple_workouts(id) ON DELETE CASCADE,
    recorded_at         TIMESTAMPTZ      NOT NULL,
    latitude            DOUBLE PRECISION NOT NULL,
    longitude           DOUBLE PRECISION NOT NULL,
    altitude_m          DOUBLE PRECISION,
    speed               DOUBLE PRECISION,
    horizontal_accuracy DOUBLE PRECISION,
    course_accuracy     DOUBLE PRECISION
);
CREATE INDEX apple_workout_route_workout_id_idx
    ON apple_workout_route (workout_id, recorded_at);

CREATE TABLE apple_daily_metrics (
    metric_name TEXT             NOT NULL,
    measured_at TIMESTAMPTZ      NOT NULL,
    source      TEXT             NOT NULL DEFAULT '',
    qty         DOUBLE PRECISION NOT NULL,
    units       TEXT             NOT NULL,
    PRIMARY KEY (metric_name, measured_at, source)
);
CREATE INDEX apple_daily_metrics_date_idx ON apple_daily_metrics (measured_at DESC);

CREATE TABLE apple_imports (
    id              BIGSERIAL   PRIMARY KEY,
    source_filename TEXT        NOT NULL,
    source_sha256   TEXT        NOT NULL UNIQUE,
    range_start     TIMESTAMPTZ,
    range_end       TIMESTAMPTZ,
    workouts_added  INT,
    metrics_added   INT,
    imported_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
