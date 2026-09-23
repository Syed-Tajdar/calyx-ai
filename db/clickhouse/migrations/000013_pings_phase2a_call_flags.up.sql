-- NULL = field was never requested (webhook path, or a row written before Phase 2A).
-- 0/1 = Phase 2A API-sync mapper requested the column. Absent on the Ringba record is stored as 0.
ALTER TABLE pings ADD COLUMN IF NOT EXISTS has_connected Nullable(UInt8);
ALTER TABLE pings ADD COLUMN IF NOT EXISTS was_blocked Nullable(UInt8);
ALTER TABLE pings ADD COLUMN IF NOT EXISTS block_reason String DEFAULT '';
ALTER TABLE pings ADD COLUMN IF NOT EXISTS was_conversion_adjusted Nullable(UInt8);
