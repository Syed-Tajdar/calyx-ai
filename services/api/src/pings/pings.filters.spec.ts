import { buildPingWhereClause } from './pings.filters';

describe('Pings publisher / campaign filters', () => {
  it('filters by exact publisher and campaign id, as parameters only', () => {
    const evil = "ID1' OR 1=1 --";
    const { clause, params } = buildPingWhereClause({ publisher: evil, campaign: 'C42' });
    expect(clause).toContain('AND publisher_id = {publisher:String}');
    expect(clause).toContain('AND campaign_id = {campaign:String}');
    expect(clause).not.toContain(evil);
    expect(params).toMatchObject({ publisher: evil, campaign: 'C42' });
  });

  it('adds nothing when the filters are absent', () => {
    const { clause } = buildPingWhereClause({});
    expect(clause).not.toContain('publisher_id');
    expect(clause).not.toContain('campaign_id');
  });
});
