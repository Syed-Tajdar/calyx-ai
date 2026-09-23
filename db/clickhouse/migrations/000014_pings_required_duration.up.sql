-- Per-call conversion requirement from Ringba ring-tree bid fields.
-- NULL = key absent on the Ringba record (no requirement sent).
-- 0 or N = key present with that value (0 is a real requirement, not "absent").
ALTER TABLE pings ADD COLUMN IF NOT EXISTS required_duration_seconds Nullable(UInt32);
ALTER TABLE pings ADD COLUMN IF NOT EXISTS max_required_duration_seconds Nullable(UInt32);
