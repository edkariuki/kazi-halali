-- Kazi Halali schema
-- Applied automatically by docker-compose on first start.

CREATE TYPE agency_status AS ENUM ('valid', 'under_verification', 'expired', 'unknown');
CREATE TYPE verdict AS ENUM ('danger', 'caution', 'likely_ok');

-- Snapshot of the NEA licensed-agency list.
CREATE TABLE agencies (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT          NOT NULL,
    normalized_name TEXT          NOT NULL,
    nea_reg_no      TEXT          NOT NULL DEFAULT '',
    status          agency_status NOT NULL DEFAULT 'unknown',
    source_url      TEXT          NOT NULL DEFAULT '',
    snapshot_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    UNIQUE (normalized_name)
);
CREATE INDEX agencies_reg_no_idx ON agencies (nea_reg_no) WHERE nea_reg_no <> '';

-- Agencies named in official warnings / "not registered" notices.
-- Only names from official sources, always with the source link.
CREATE TABLE flagged_entities (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT        NOT NULL,
    normalized_name TEXT        NOT NULL,
    reason          TEXT        NOT NULL,
    source_title    TEXT        NOT NULL,
    source_url      TEXT        NOT NULL,
    published_at    DATE,
    UNIQUE (normalized_name, source_url)
);

-- Every check a user runs. The full result is stored as JSONB so the share
-- page can be re-rendered exactly; a few columns are lifted out for stats.
CREATE TABLE checks (
    id           TEXT        PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    verdict      verdict     NOT NULL,
    agency_name  TEXT        NOT NULL DEFAULT '',
    destination  TEXT        NOT NULL DEFAULT '',
    had_image    BOOLEAN     NOT NULL DEFAULT false,
    result       JSONB       NOT NULL
);
CREATE INDEX checks_created_idx ON checks (created_at DESC);
CREATE INDEX checks_verdict_idx ON checks (verdict);
