ALTER TABLE pings DROP COLUMN IF EXISTS has_connected;
ALTER TABLE pings DROP COLUMN IF EXISTS was_blocked;
ALTER TABLE pings DROP COLUMN IF EXISTS block_reason;
ALTER TABLE pings DROP COLUMN IF EXISTS was_conversion_adjusted;
