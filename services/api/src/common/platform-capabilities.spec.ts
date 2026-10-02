import {
  conversionFilterSql,
  conversionState,
} from './platform-capabilities';

describe('conversionState', () => {
  it('stamps converted only when the flag is true and revenue is above zero', () => {
    expect(
      conversionState({ platform: 'retreaver', hasConverted: true, revenue: 34.13 }),
    ).toBe('converted');
    expect(conversionState({ platform: 'ringba', hasConverted: true, revenue: 0.01 })).toBe(
      'converted',
    );
  });

  it('puts a true flag with $0 in converted_zero', () => {
    expect(conversionState({ platform: 'retreaver', hasConverted: true, revenue: 0 })).toBe(
      'converted_zero',
    );
  });

  it('stamps not_converted when the platform reports and the flag is false', () => {
    expect(conversionState({ platform: 'ringba', hasConverted: false, revenue: 12 })).toBe(
      'not_converted',
    );
  });

  it('stamps not_reported for platforms that do not report conversion', () => {
    expect(conversionState({ platform: 'trackdrive', hasConverted: false, revenue: 10 })).toBe(
      'not_reported',
    );
    expect(conversionState({ platform: 'callgrid', hasConverted: true, revenue: 10 })).toBe(
      'not_reported',
    );
  });
});

describe('conversionFilterSql', () => {
  it('Converted is flag and revenue > 0, matching write-back isConverted', () => {
    const sql = conversionFilterSql('converted', 'latest');
    expect(sql).toContain('argMax(has_converted, received_at) = 1');
    expect(sql).toContain('argMax(revenue, received_at) > 0');
    expect(sql).toContain("platform IN ('ringba', 'retreaver')");
  });

  it('Not converted is the flag false, so $0 conversions stay out of that filter', () => {
    const sql = conversionFilterSql('unconverted', 'raw');
    expect(sql).toContain('has_converted = 0');
    expect(sql).not.toContain('revenue = 0');
  });

  it('Should have converted stays not_converted plus revenue or duration', () => {
    const sql = conversionFilterSql('should_convert', 'raw');
    expect(sql).toContain('has_converted = 0');
    expect(sql).toContain('revenue > 0');
    expect(sql).toContain('duration_seconds >= required_duration_seconds');
  });
});
