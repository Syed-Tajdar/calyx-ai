-- Early refusals, including WRITEBACK_ENABLED=false, are audited as rejected.
-- The original check only allowed confirmed, pending, and failed, so those
-- inserts were rejected by Postgres and the row never landed.

ALTER TABLE writeback_audit DROP CONSTRAINT writeback_audit_outcome_check;
ALTER TABLE writeback_audit
    ADD CONSTRAINT writeback_audit_outcome_check
    CHECK (outcome IN ('confirmed', 'pending', 'failed', 'rejected'));
