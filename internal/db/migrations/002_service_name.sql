-- Phase 5.1: stable service identity (name unique per project).
ALTER TABLE services ADD COLUMN IF NOT EXISTS name TEXT;

UPDATE services
SET name = 'svc-' || replace(substr(id::text, 1, 8), '-', '')
WHERE name IS NULL;

ALTER TABLE services ALTER COLUMN name SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_services_project_id_name ON services (project_id, name);
