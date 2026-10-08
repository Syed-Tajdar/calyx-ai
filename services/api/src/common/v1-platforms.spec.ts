import { platformScopeSql, V1_PLATFORM_SQL } from './v1-platforms';
import { buildPingWhereClause } from '../pings/pings.filters';

describe('v1 platform scope', () => {
  it('limits All to ringba and retreaver without a platform param', () => {
    const scope = platformScopeSql(undefined, true);
    expect(scope.sql).toBe(V1_PLATFORM_SQL);
    expect(scope.sql).toContain("'ringba'");
    expect(scope.sql).toContain("'retreaver'");
    expect(scope.params).toEqual({});
  });

  it('keeps a chosen platform and leaves All open when the flag is off', () => {
    expect(platformScopeSql('ringba', true).sql).toBe('AND platform = {platform:String}');
    expect(platformScopeSql(undefined, false).sql).toBe('');
  });

  it('adds the same clause to the ping list and export filter', () => {
    const { clause } = buildPingWhereClause({ v1_platforms: true });
    expect(clause).toContain(V1_PLATFORM_SQL.trim());
    const chosen = buildPingWhereClause({ platform: 'ringba', v1_platforms: true });
    expect(chosen.clause).toContain('platform = {platform:String}');
    expect(chosen.clause).not.toContain('IN (');
  });
});
