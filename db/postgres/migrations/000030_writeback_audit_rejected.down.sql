ALTER TABLE writeback_audit DROP CONSTRAINT writeback_audit_outcome_check;
ALTER TABLE writeback_audit
    ADD CONSTRAINT writeback_audit_outcome_check
    CHECK (outcome IN ('confirmed', 'pending', 'failed'));
