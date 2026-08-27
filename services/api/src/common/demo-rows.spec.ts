import * as fs from 'fs';
import * as path from 'path';
import {
  apiSyncExclusionSql,
  demoExclusionSql,
  API_SYNC_ID_PREFIX,
} from './demo-rows';

/**
 * Read paths that MUST exclude apisync-% rows via apiSyncExclusionSql (or deprecated alias).
 * If a new aggregate/list query is added without the helper, this test fails.
 */
const REQUIRED_API_SYNC_EXCLUSION_FILES = [
  'analytics/analytics.service.ts',
  'coverage/coverage.service.ts',
  'scoring/scoring.service.ts',
  'compliance/compliance.service.ts',
  'pings/pings.filters.ts',
] as const;

describe('apiSyncExclusionSql', () => {
  it('excludes apisync-% by default', () => {
    expect(apiSyncExclusionSql('external_call_id')).toBe(
      `AND external_call_id NOT LIKE '${API_SYNC_ID_PREFIX}%'`,
    );
    expect(apiSyncExclusionSql('call_id')).toContain("NOT LIKE 'apisync-%'");
  });

  it('returns empty when includeApiSync is true', () => {
    expect(apiSyncExclusionSql('external_call_id', true)).toBe('');
  });

  it('rejects unsafe column identifiers', () => {
    expect(() => apiSyncExclusionSql("id; DROP TABLE pings")).toThrow(/invalid/);
  });

  it('deprecated demoExclusionSql alias still works', () => {
    expect(demoExclusionSql('external_call_id')).toBe(apiSyncExclusionSql('external_call_id'));
  });
});

describe('api-sync exclusion anti-drift', () => {
  const srcRoot = path.join(__dirname, '..');

  it('every required read path imports and calls apiSyncExclusionSql (or demoExclusionSql alias)', () => {
    for (const rel of REQUIRED_API_SYNC_EXCLUSION_FILES) {
      const full = path.join(srcRoot, rel);
      expect(fs.existsSync(full)).toBe(true);
      const content = fs.readFileSync(full, 'utf8');
      expect(content).toContain("from '../common/demo-rows'");
      expect(
        /apiSyncExclusionSql\(|demoExclusionSql\(/.test(content),
      ).toBe(true);
      // No duplicated inline predicate — must go through the helper.
      expect(content).not.toMatch(/NOT LIKE ['"]apisync-%['"]/);
    }
  });

  it('fails loudly if analytics.service omits the shared predicate (regression guard)', () => {
    const analytics = fs.readFileSync(
      path.join(srcRoot, 'analytics/analytics.service.ts'),
      'utf8',
    );
    expect(analytics).toMatch(
      /(apiSyncExclusionSql|demoExclusionSql)\(\s*['"]external_call_id['"]/,
    );
  });
});
