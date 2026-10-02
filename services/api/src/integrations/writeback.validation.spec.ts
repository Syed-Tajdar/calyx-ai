import { convertWritebackSchema } from './writeback.validation';

const id = '12345678';

describe('convertWritebackSchema amount', () => {
  it('accepts 26 and 1000', () => {
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: 26 }).success).toBe(
      true,
    );
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: 1000 }).success).toBe(
      true,
    );
  });

  it('rejects 0, negative, NaN, and a string', () => {
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: 0 }).success).toBe(
      false,
    );
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: -1 }).success).toBe(
      false,
    );
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: Number.NaN }).success).toBe(
      false,
    );
    expect(convertWritebackSchema.safeParse({ inbound_call_id: id, amount: '26' }).success).toBe(
      false,
    );
  });
});
