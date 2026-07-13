-- Platform-declared duplicate lead (from webhook payload) — distinct from ingestion Redis dedup (is_duplicate).
ALTER TABLE pings ADD COLUMN IF NOT EXISTS platform_duplicate UInt8 DEFAULT 0;
