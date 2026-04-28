CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(64) NOT NULL,
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    quietness_level SMALLINT NOT NULL CHECK (quietness_level BETWEEN 1 AND 5),
    place_name VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS confirmations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT confirmations_report_user_unique UNIQUE (report_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_reports_location_gist ON reports USING GIST (location);
CREATE INDEX IF NOT EXISTS idx_reports_created_at ON reports (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_confirmations_report_id ON confirmations (report_id);
CREATE INDEX IF NOT EXISTS idx_confirmations_user_id ON confirmations (user_id);

CREATE OR REPLACE FUNCTION temporal_decay(report_created_at TIMESTAMPTZ)
RETURNS DOUBLE PRECISION
LANGUAGE SQL
STABLE
AS $$
    SELECT
        CASE
            WHEN report_created_at < NOW() - INTERVAL '24 hours' THEN 0.0
            WHEN report_created_at >= NOW() - INTERVAL '2 hours' THEN 1.0
            ELSE
                1.0 - (
                    (EXTRACT(EPOCH FROM (NOW() - report_created_at)) - 7200.0)
                    / 79200.0
                ) * 0.5
        END
$$;
