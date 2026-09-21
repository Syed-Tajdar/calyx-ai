import * as fs from 'fs';
import * as path from 'path';
import {
  apiSyncExclusionSql,
  demoExclusionSql,
  testDataExclusionSql,
  TEST_DATA_ID_PREFIXES,
} from './demo-rows';

/**
 * Read paths that MUST exclude test-data prefixes via testDataExclusionSql
 * (or a deprecated alias). If a new aggregate/list query is added without the
 * helper, this test fails.
 */
const REQUIRED_API_SYNC_EXCLUSION_FILES = [
  'analytics/analytics.service.ts',
  'coverage/coverage.service.ts',
  'scoring/scoring.service.ts',
  'compliance/compliance.service.ts',
  'pings/pings.filters.ts',
] as const;

describe('testDataExclusionSql', () => {
  it('excludes apisync-, demo-, and smoke- by default', () => {
    const sql = testDataExclusionSql('external_call_id');
    for (const prefix of TEST_DATA_ID_PREFIXES) {
      expect(sql).toContain(`NOT LIKE '${prefix}%'`);
    }
    expect(TEST_DATA_ID_PREFIXES).toEqual(['apisync-', 'demo-', 'smoke-']);
    expect(apiSyncExclusionSql('call_id')).toContain("NOT LIKE 'apisync-%'");
    expect(apiSyncExclusionSql('call_id')).toContain("NOT LIKE 'demo-%'");
    expect(apiSyncExclusionSql('call_id')).toContain("NOT LIKE 'smoke-%'");
  });

  it('returns empty when show_demo includes all three families', () => {
    expect(testDataExclusionSql('external_call_id', true)).toBe('');
    expect(apiSyncExclusionSql('external_call_id', true)).toBe('');
  });

  it('rejects unsafe column identifiers', () => {
    expect(() => testDataExclusionSql("id; DROP TABLE pings")).toThrow(/invalid/);
  });

  it('deprecated aliases match the three-prefix helper', () => {
    expect(demoExclusionSql('external_call_id')).toBe(testDataExclusionSql('external_call_id'));
    expect(apiSyncExclusionSql('external_call_id')).toBe(testDataExclusionSql('external_call_id'));
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
        /(testDataExclusionSql|apiSyncExclusionSql|demoExclusionSql)\(/.test(content),
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
      /(testDataExclusionSql|apiSyncExclusionSql|demoExclusionSql)\(\s*['"]external_call_id['"]/,
    );
  });
});
