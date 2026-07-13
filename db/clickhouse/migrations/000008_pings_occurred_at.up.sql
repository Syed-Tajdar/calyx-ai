ALTER TABLE pings ADD COLUMN IF NOT EXISTS occurred_at DateTime('UTC') DEFAULT toDateTime(0);
