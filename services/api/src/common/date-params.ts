import { z } from 'zod';

/** ISO-8601 datetime (or date-only) — rejects bare garbage like `from=garbage`. */
export const isoDateTimeString = z
  .string()
  .refine((v) => !Number.isNaN(Date.parse(v)), {
    message: 'must be a valid ISO-8601 date/datetime',
  });

export const optionalIsoDate = isoDateTimeString.optional();
