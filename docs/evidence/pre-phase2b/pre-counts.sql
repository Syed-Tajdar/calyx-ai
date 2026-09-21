SELECT 'old_smoke_rows' AS k, toString(count()) AS v
FROM default.pings
WHERE external_call_id IN (
  '0e92eece-73fb-4d06-b9c7-858862aca994',
  'ea023ec0-927b-43b9-af44-8befd5afe113',
  'ba472c0e-3212-4013-a760-c197b3877617',
  'cb51ea8b-6234-41cc-8697-3aad6729381a',
  'dbbdef75-8c22-4b08-bd82-5bdbb1bd08fe',
  '128fb18f-3738-4ddc-9b51-dd367b302778',
  '1906f701-2bdb-4b56-9f9c-f5dca66334af',
  '195c1e62-d264-4d6f-a9e1-a28347152e3f',
  '7afabab6-90d8-4a14-a255-4b648ded24b1',
  'cb45766e-04d7-4510-b117-075bbcb25305',
  'db04ac18-cce8-44de-ae41-1b9b896c8856',
  'f2b1d903-b545-4b14-8df6-dc6154060154',
  '1f149b5f-b772-4216-b397-fd7d8e65f920',
  'b9ff1104-aa1f-4abd-9a7a-00f69f68e240'
);

SELECT 'old_smoke_calls' AS k, toString(uniqExact(external_call_id)) AS v
FROM default.pings
WHERE external_call_id IN (
  '0e92eece-73fb-4d06-b9c7-858862aca994',
  'ea023ec0-927b-43b9-af44-8befd5afe113',
  'ba472c0e-3212-4013-a760-c197b3877617',
  'cb51ea8b-6234-41cc-8697-3aad6729381a',
  'dbbdef75-8c22-4b08-bd82-5bdbb1bd08fe',
  '128fb18f-3738-4ddc-9b51-dd367b302778',
  '1906f701-2bdb-4b56-9f9c-f5dca66334af',
  '195c1e62-d264-4d6f-a9e1-a28347152e3f',
  '7afabab6-90d8-4a14-a255-4b648ded24b1',
  'cb45766e-04d7-4510-b117-075bbcb25305',
  'db04ac18-cce8-44de-ae41-1b9b896c8856',
  'f2b1d903-b545-4b14-8df6-dc6154060154',
  '1f149b5f-b772-4216-b397-fd7d8e65f920',
  'b9ff1104-aa1f-4abd-9a7a-00f69f68e240'
);

SELECT 'old_smoke_argmax_revenue' AS k, toString(sum(revenue)) AS v
FROM (
  SELECT argMax(revenue, received_at) AS revenue
  FROM default.pings
  WHERE external_call_id IN (
    '0e92eece-73fb-4d06-b9c7-858862aca994',
    'ea023ec0-927b-43b9-af44-8befd5afe113',
    'ba472c0e-3212-4013-a760-c197b3877617',
    'cb51ea8b-6234-41cc-8697-3aad6729381a',
    'dbbdef75-8c22-4b08-bd82-5bdbb1bd08fe',
    '128fb18f-3738-4ddc-9b51-dd367b302778',
    '1906f701-2bdb-4b56-9f9c-f5dca66334af',
    '195c1e62-d264-4d6f-a9e1-a28347152e3f',
    '7afabab6-90d8-4a14-a255-4b648ded24b1',
    'cb45766e-04d7-4510-b117-075bbcb25305',
    'db04ac18-cce8-44de-ae41-1b9b896c8856',
    'f2b1d903-b545-4b14-8df6-dc6154060154',
    '1f149b5f-b772-4216-b397-fd7d8e65f920',
    'b9ff1104-aa1f-4abd-9a7a-00f69f68e240'
  )
  GROUP BY external_call_id
);

SELECT 'apisync_rows' AS k, toString(count()) AS v FROM default.pings WHERE external_call_id LIKE 'apisync-%';
SELECT 'demo_rows' AS k, toString(count()) AS v FROM default.pings WHERE external_call_id LIKE 'demo-%';
SELECT 'smoke_rows' AS k, toString(count()) AS v FROM default.pings WHERE external_call_id LIKE 'smoke-%';
SELECT 'smoke_calls' AS k, toString(uniqExact(external_call_id)) AS v FROM default.pings WHERE external_call_id LIKE 'smoke-%';
SELECT 'rgb_api_sync_rows' AS k, toString(count()) AS v FROM default.pings WHERE external_call_id LIKE 'RGB%' AND sync_path = 'api_sync';
SELECT 'rgb_rows' AS k, toString(count()) AS v FROM default.pings WHERE external_call_id LIKE 'RGB%';

SELECT 'kpi_pings' AS k, toString(count()) AS v
FROM (
  SELECT platform, external_call_id
  FROM default.pings
  WHERE external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
);

SELECT 'kpi_revenue' AS k, toString(sumIf(revenue, platform_duplicate = 0)) AS v
FROM (
  SELECT
    argMax(revenue, received_at) AS revenue,
    argMax(platform_duplicate, received_at) AS platform_duplicate
  FROM default.pings
  WHERE external_call_id != ''
    AND external_call_id NOT LIKE 'apisync-%'
    AND external_call_id NOT LIKE 'demo-%'
    AND external_call_id NOT LIKE 'smoke-%'
  GROUP BY platform, external_call_id
);
