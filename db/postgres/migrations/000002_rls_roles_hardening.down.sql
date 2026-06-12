REVOKE EXECUTE ON FUNCTION authenticate_user(TEXT, TEXT) FROM callloom_api;
DROP FUNCTION IF EXISTS authenticate_user(TEXT, TEXT);

REVOKE EXECUTE ON FUNCTION lookup_integration_by_token(TEXT) FROM callloom_ingestion;
REVOKE SELECT ON tenants, users, integrations FROM callloom_api;

ALTER TABLE tenants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE integrations NO FORCE ROW LEVEL SECURITY;

DROP ROLE IF EXISTS callloom_ingestion;
DROP ROLE IF EXISTS callloom_api;
