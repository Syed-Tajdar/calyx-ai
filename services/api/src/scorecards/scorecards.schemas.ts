import { z } from 'zod';

export const scorecardRuleSchema = z.object({
  id: z.string().min(1),
  weight: z.number().min(0).max(1),
  question: z.string().min(1),
});

export const createScorecardSchema = z.object({
  name: z.string().min(1).max(128),
  vertical: z.string().min(1).max(64).default('general'),
  rules: z.array(scorecardRuleSchema).min(1),
  is_active: z.boolean().optional().default(true),
});

export const updateScorecardSchema = z.object({
  name: z.string().min(1).max(128).optional(),
  vertical: z.string().min(1).max(64).optional(),
  rules: z.array(scorecardRuleSchema).min(1).optional(),
  is_active: z.boolean().optional(),
});

export type CreateScorecardInput = z.output<typeof createScorecardSchema>;
export type UpdateScorecardInput = z.output<typeof updateScorecardSchema>;
