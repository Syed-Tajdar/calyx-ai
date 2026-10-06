import { z } from 'zod';

/**
 * Amount is finite and > 0. There is no upper cap — a $25 UI limit was removed
 * (stage 1–3 A1). NaN, strings, zero, and negatives fail this schema.
 */
export const convertWritebackSchema = z.object({
  inbound_call_id: z.string().min(8).max(128),
  amount: z
    .number({
      required_error: 'amount must be a positive number',
      invalid_type_error: 'amount must be a positive number',
    })
    .positive({ message: 'amount must be a positive number' }),
});

export const voidWritebackSchema = z.object({
  inbound_call_id: z.string().min(8).max(128),
});
