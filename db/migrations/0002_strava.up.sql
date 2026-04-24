-- Batch 3 — Strava OAuth2 + activity collector

CREATE TABLE strava_athletes (
    strava_athlete_id   BIGINT      PRIMARY KEY,
    username            TEXT,
    firstname           TEXT,
    lastname            TEXT,
    city                TEXT,
    country             TEXT,
    sex                 TEXT,
    weight_kg           DOUBLE PRECISION,
    ftp_watts           INT,
    profile_url         TEXT,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per athlete; access_token is short-lived, refresh_token is long-lived.
CREATE TABLE strava_tokens (
    strava_athlete_id   BIGINT      PRIMARY KEY REFERENCES strava_athletes(strava_athlete_id) ON DELETE CASCADE,
    access_token        TEXT        NOT NULL,
    refresh_token       TEXT        NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One-to-one for v1: one Telegram chat maps to one Strava athlete.
-- ON CONFLICT (telegram_chat_id) DO UPDATE moves the link if the user re-logins
-- from a different chat or with a different Strava account.
CREATE TABLE telegram_strava_links (
    telegram_chat_id    BIGINT      PRIMARY KEY,
    strava_athlete_id   BIGINT      NOT NULL REFERENCES strava_athletes(strava_athlete_id) ON DELETE CASCADE,
    linked_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX telegram_strava_links_athlete_idx ON telegram_strava_links (strava_athlete_id);

-- Activity summary fields from GET /athlete/activities.
-- strava_activity_id is the PK — globally unique on Strava.
CREATE TABLE strava_activities (
    strava_activity_id  BIGINT      PRIMARY KEY,
    strava_athlete_id   BIGINT      NOT NULL REFERENCES strava_athletes(strava_athlete_id) ON DELETE CASCADE,
    name                TEXT        NOT NULL,
    sport_type          TEXT        NOT NULL,
    workout_type        INT,
    start_at            TIMESTAMPTZ NOT NULL,
    start_at_local      TIMESTAMPTZ,
    timezone            TEXT,
    distance_m          DOUBLE PRECISION,
    moving_time_s       INT,
    elapsed_time_s      INT,
    elevation_gain_m    DOUBLE PRECISION,
    average_speed_mps   DOUBLE PRECISION,
    max_speed_mps       DOUBLE PRECISION,
    average_heartrate   DOUBLE PRECISION,
    max_heartrate       DOUBLE PRECISION,
    average_watts       DOUBLE PRECISION,
    average_cadence     DOUBLE PRECISION,
    suffer_score        INT,
    trainer             BOOLEAN,
    commute             BOOLEAN,
    payload             JSONB       NOT NULL,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX strava_activities_athlete_start_idx ON strava_activities (strava_athlete_id, start_at DESC);

-- Cursor for incremental sync: stores the start_at of the last fetched activity.
-- Upserted after each successful /pull.
CREATE TABLE strava_sync_state (
    strava_athlete_id   BIGINT      PRIMARY KEY REFERENCES strava_athletes(strava_athlete_id) ON DELETE CASCADE,
    last_synced_at      TIMESTAMPTZ,
    last_activity_at    TIMESTAMPTZ
);

-- Single row tracking Strava API quota consumed in the current window.
-- Refreshed from X-RateLimit-* response headers after every API call.
CREATE TABLE strava_rate_limit (
    id                  INT         PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    short_limit         INT         NOT NULL DEFAULT 100,
    short_usage         INT         NOT NULL DEFAULT 0,
    daily_limit         INT         NOT NULL DEFAULT 1000,
    daily_usage         INT         NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO strava_rate_limit (id) VALUES (1);
