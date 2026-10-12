import { MockAdSpendSource } from './adspend.source';

describe('MockAdSpendSource', () => {
  const previous = process.env.V1_ONLY;

  afterEach(() => {
    if (previous === undefined) delete process.env.V1_ONLY;
    else process.env.V1_ONLY = previous;
  });

  it('returns no spend in v1 so Reports cannot show invented money', async () => {
    delete process.env.V1_ONLY;
    const rows = await new MockAdSpendSource().getSpend('11111111-1111-1111-1111-111111111111');
    expect(rows).toEqual([]);
  });

  it('returns the demo 250+120 when v1 is off', async () => {
    process.env.V1_ONLY = 'false';
    const rows = await new MockAdSpendSource().getSpend('11111111-1111-1111-1111-111111111111');
    expect(rows.reduce((sum, row) => sum + row.amount, 0)).toBe(370);
  });
});
