CREATE TABLE areas (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    geometry JSONB NOT NULL,
    source JSONB NOT NULL,
    period_from DATE NOT NULL,
    period_to DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    generation INTEGER NOT NULL DEFAULT 1 CHECK (generation > 0),
    shown_result_version TEXT,
    shown_job_id TEXT,
    active_job_id TEXT,
    CHECK (period_from <= period_to)
);

CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    area_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    stage TEXT,
    period_from DATE NOT NULL,
    period_to DATE NOT NULL,
    error_code TEXT,
    error_message TEXT,
    error_retryable BOOLEAN,
    result_version TEXT,
    area_generation INTEGER NOT NULL CHECK (area_generation > 0),
    input_revision TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK (period_from <= period_to)
);

CREATE TABLE analysis_results (
    area_id TEXT NOT NULL,
    result_version TEXT NOT NULL,
    period_from DATE NOT NULL,
    period_to DATE NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    schema_version TEXT NOT NULL,
    feature_profile TEXT NOT NULL,
    model_version TEXT NOT NULL,
    method TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('normal', 'candidate', 'confirmed', 'insufficient_data')),
    severity TEXT,
    input_revision TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    series JSONB NOT NULL DEFAULT '[]'::jsonb,
    weather JSONB NOT NULL DEFAULT '[]'::jsonb,
    provenance JSONB NOT NULL DEFAULT '{}'::jsonb,
    limitations JSONB NOT NULL DEFAULT '[]'::jsonb,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    PRIMARY KEY (area_id, result_version),
    CHECK (period_from <= period_to),
    CHECK (severity IS NULL OR severity IN ('none', 'moderate', 'high'))
);

CREATE INDEX jobs_area_created_at_idx ON jobs (area_id, created_at DESC);
CREATE INDEX results_area_computed_at_idx ON analysis_results (area_id, computed_at DESC);
CREATE UNIQUE INDEX jobs_one_active_per_area_idx
    ON jobs (area_id)
    WHERE status IN ('queued', 'running');
