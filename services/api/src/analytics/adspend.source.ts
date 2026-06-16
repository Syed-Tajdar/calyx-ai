export interface AdSpendRow {
  campaign_id: string;
  spend_date: string;
  amount: number;
}

export interface AdSpendSource {
  getSpend(tenantId: string): Promise<AdSpendRow[]>;
}

export class MockAdSpendSource implements AdSpendSource {
  async getSpend(_tenantId: string): Promise<AdSpendRow[]> {
    return [
      { campaign_id: 'camp_google', spend_date: new Date().toISOString().slice(0, 10), amount: 250 },
      { campaign_id: 'camp_bing', spend_date: new Date().toISOString().slice(0, 10), amount: 120 },
    ];
  }
}

export class ManualAdSpendSource implements AdSpendSource {
  constructor(private readonly query: (tenantId: string) => Promise<AdSpendRow[]>) {}

  async getSpend(tenantId: string): Promise<AdSpendRow[]> {
    return this.query(tenantId);
  }
}

export function createAdSpendSource(
  sourceName: string,
  manualQuery: (tenantId: string) => Promise<AdSpendRow[]>,
): AdSpendSource {
  if (sourceName === 'manual') {
    return new ManualAdSpendSource(manualQuery);
  }
  return new MockAdSpendSource();
}
