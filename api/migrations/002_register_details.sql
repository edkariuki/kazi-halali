-- Registered contact details and register dates, used to spot agencies being
-- impersonated and to tell users how current our copy of the register is.
ALTER TABLE agencies ADD COLUMN IF NOT EXISTS service_type TEXT NOT NULL DEFAULT '';
ALTER TABLE agencies ADD COLUMN IF NOT EXISTS email        TEXT NOT NULL DEFAULT '';
ALTER TABLE agencies ADD COLUMN IF NOT EXISTS website      TEXT NOT NULL DEFAULT '';
ALTER TABLE agencies ADD COLUMN IF NOT EXISTS published_at DATE;