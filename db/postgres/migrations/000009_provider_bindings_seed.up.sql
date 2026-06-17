INSERT INTO tenants (id, name) VALUES
    ('22222222-2222-2222-2222-222222222222', 'Call Loom Dev Tenant B')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (tenant_id, email, password_hash, role) VALUES
    ('22222222-2222-2222-2222-222222222222',
     'tenantb@callloom.dev',
     crypt('callloom_dev', gen_salt('bf')),
     'admin')
ON CONFLICT (tenant_id, email) DO NOTHING;

INSERT INTO tenant_provider_bindings (tenant_id, domain, provider_id, config_json, priority) VALUES
    ('11111111-1111-1111-1111-111111111111', 'attribution', 'mock',
     '{"variant":"tenant_a","label":"A"}'::jsonb, 0),
    ('22222222-2222-2222-2222-222222222222', 'attribution', 'mock',
     '{"variant":"tenant_b","label":"B"}'::jsonb, 0)
ON CONFLICT (tenant_id, domain, provider_id) DO UPDATE
    SET config_json = EXCLUDED.config_json,
        updated_at = NOW();
