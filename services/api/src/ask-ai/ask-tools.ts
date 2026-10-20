import { z } from 'zod';

/**
 * Ask AI tools (v2 Phase 2A). The model may only choose one of these tools and its arguments;
 * the API runs the query itself with the tenant from the JWT. There is no free SQL and no
 * tenant argument: a tool cannot reach another tenant's data.
 *
 * Every schema is OpenAI "strict" (all properties required, additionalProperties false);
 * optional values are expressed as nullable.
 */

export const DIMENSIONS = ['platform', 'campaign', 'publisher', 'buyer', 'state', 'day'] as const;
export type Dimension = (typeof DIMENSIONS)[number];

export const METRICS = [
  'calls',
  'converted',
  'conversion_rate',
  'revenue',
  'payout',
  'profit',
  'revenue_per_call',
] as const;
export type Metric = (typeof METRICS)[number];

export const PLATFORMS = ['all', 'ringba', 'retreaver'] as const;

const isoDate = z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'date must be YYYY-MM-DD');

const filterSchema = z
  .object({
    dimension: z.enum(['campaign', 'publisher', 'buyer', 'state']),
    value: z.string().min(1).max(200),
  })
  .strict();

export const metricsArgsSchema = z
  .object({
    group_by: z.enum(['none', ...DIMENSIONS]),
    from: isoDate,
    to: isoDate,
    platform: z.enum(PLATFORMS),
    filter: filterSchema.nullable(),
    order_by: z.enum(METRICS),
    order: z.enum(['asc', 'desc']),
    limit: z.number().int().min(1).max(50),
    min_calls: z.number().int().min(0).max(100000),
  })
  .strict();
export type MetricsArgs = z.infer<typeof metricsArgsSchema>;

const periodSchema = z.object({ from: isoDate, to: isoDate }).strict();

export const compareArgsSchema = z
  .object({
    group_by: z.enum(['none', 'platform', 'campaign', 'publisher', 'buyer', 'state']),
    current: periodSchema,
    previous: periodSchema,
    platform: z.enum(PLATFORMS),
    filter: filterSchema.nullable(),
    order_by: z.enum(['calls_change_pct', 'revenue_change_pct', 'conversion_rate_change', 'revenue', 'calls']),
    order: z.enum(['asc', 'desc']),
    limit: z.number().int().min(1).max(50),
    min_calls: z.number().int().min(0).max(100000),
  })
  .strict();
export type CompareArgs = z.infer<typeof compareArgsSchema>;

export const listValuesArgsSchema = z
  .object({
    dimension: z.enum(['campaign', 'publisher', 'buyer', 'state']),
    search: z.string().max(100).nullable(),
    from: isoDate,
    to: isoDate,
    limit: z.number().int().min(1).max(50),
  })
  .strict();
export type ListValuesArgs = z.infer<typeof listValuesArgsSchema>;

export type ToolName = 'metrics' | 'compare_periods' | 'list_values';

export const TOOL_ARG_SCHEMAS: Record<ToolName, z.ZodTypeAny> = {
  metrics: metricsArgsSchema,
  compare_periods: compareArgsSchema,
  list_values: listValuesArgsSchema,
};

/** Longest date range a tool may read, in days (keeps queries bounded). */
export const MAX_RANGE_DAYS = 400;

const nullable = (schema: Record<string, unknown>) => ({ anyOf: [schema, { type: 'null' }] });

const filterJson = nullable({
  type: 'object',
  additionalProperties: false,
  required: ['dimension', 'value'],
  properties: {
    dimension: { type: 'string', enum: ['campaign', 'publisher', 'buyer', 'state'] },
    value: {
      type: 'string',
      description:
        'Exact value as returned by list_values (state: 2-letter code such as "TX"). Use list_values first when unsure.',
    },
  },
});

const periodJson = {
  type: 'object',
  additionalProperties: false,
  required: ['from', 'to'],
  properties: {
    from: { type: 'string', description: 'First day, YYYY-MM-DD, Eastern time, inclusive' },
    to: { type: 'string', description: 'Last day, YYYY-MM-DD, Eastern time, inclusive' },
  },
};

/** Tool definitions in the OpenAI Responses API format (strict function tools). */
export const OPENAI_TOOLS = [
  {
    type: 'function',
    name: 'metrics',
    description:
      'Call metrics from Calyx (deduplicated calls, same numbers as the dashboard) for a date range in Eastern time. ' +
      'Optionally grouped by one dimension and filtered by one value. Returns rows with calls, converted (converted with revenue > 0), ' +
      'conversion_rate_pct (100 × converted / calls), revenue, payout, profit (revenue - payout) and revenue_per_call, in US dollars.',
    strict: true,
    parameters: {
      type: 'object',
      additionalProperties: false,
      required: ['group_by', 'from', 'to', 'platform', 'filter', 'order_by', 'order', 'limit', 'min_calls'],
      properties: {
        group_by: { type: 'string', enum: ['none', ...DIMENSIONS] },
        from: { type: 'string', description: 'First day, YYYY-MM-DD, Eastern time, inclusive' },
        to: { type: 'string', description: 'Last day, YYYY-MM-DD, Eastern time, inclusive' },
        platform: { type: 'string', enum: [...PLATFORMS] },
        filter: filterJson,
        order_by: { type: 'string', enum: [...METRICS] },
        order: { type: 'string', enum: ['asc', 'desc'] },
        limit: { type: 'integer', description: '1 to 50 rows' },
        min_calls: {
          type: 'integer',
          description: 'Leave out groups with fewer calls (use about 10 for "lowest/highest rate" questions; 0 otherwise)',
        },
      },
    },
  },
  {
    type: 'function',
    name: 'compare_periods',
    description:
      'Compare two date ranges (Eastern time), optionally grouped by one dimension. Returns current and previous values ' +
      'for calls, converted, conversion_rate_pct and revenue, plus calls_change_pct, revenue_change_pct and conversion_rate_change (percentage points).',
    strict: true,
    parameters: {
      type: 'object',
      additionalProperties: false,
      required: ['group_by', 'current', 'previous', 'platform', 'filter', 'order_by', 'order', 'limit', 'min_calls'],
      properties: {
        group_by: { type: 'string', enum: ['none', 'platform', 'campaign', 'publisher', 'buyer', 'state'] },
        current: periodJson,
        previous: periodJson,
        platform: { type: 'string', enum: [...PLATFORMS] },
        filter: filterJson,
        order_by: {
          type: 'string',
          enum: ['calls_change_pct', 'revenue_change_pct', 'conversion_rate_change', 'revenue', 'calls'],
        },
        order: { type: 'string', enum: ['asc', 'desc'] },
        limit: { type: 'integer', description: '1 to 50 rows' },
        min_calls: { type: 'integer', description: 'Minimum calls in the previous period for a group to be included' },
      },
    },
  },
  {
    type: 'function',
    name: 'list_values',
    description:
      'List the exact names that exist for a dimension in a date range (campaigns, publishers, buyers, states), ' +
      'optionally filtered by a search text. Use it to resolve a name the user typed before filtering.',
    strict: true,
    parameters: {
      type: 'object',
      additionalProperties: false,
      required: ['dimension', 'search', 'from', 'to', 'limit'],
      properties: {
        dimension: { type: 'string', enum: ['campaign', 'publisher', 'buyer', 'state'] },
        search: { anyOf: [{ type: 'string' }, { type: 'null' }] },
        from: { type: 'string', description: 'YYYY-MM-DD, Eastern time' },
        to: { type: 'string', description: 'YYYY-MM-DD, Eastern time' },
        limit: { type: 'integer', description: '1 to 50' },
      },
    },
  },
] as const;

/** Final answer format (OpenAI Structured Outputs, strict). */
export const ANSWER_FORMAT = {
  type: 'json_schema',
  name: 'calyx_answer',
  strict: true,
  schema: {
    type: 'object',
    additionalProperties: false,
    required: ['answer', 'cannot_answer'],
    properties: {
      answer: {
        type: 'string',
        description: 'Short plain-English answer using only numbers returned by the tools.',
      },
      cannot_answer: {
        type: 'boolean',
        description: 'true when the tools returned no data or the question cannot be answered from Calyx data.',
      },
    },
  },
} as const;

export const answerSchema = z.object({ answer: z.string(), cannot_answer: z.boolean() }).strict();

export function daysBetween(from: string, to: string): number {
  const a = Date.parse(`${from}T00:00:00Z`);
  const b = Date.parse(`${to}T00:00:00Z`);
  return Math.round((b - a) / 86_400_000);
}

/** Throws on a reversed or too-long range. */
export function assertRange(from: string, to: string): void {
  const days = daysBetween(from, to);
  if (!Number.isFinite(days) || days < 0) throw new Error(`invalid range ${from}..${to}`);
  if (days > MAX_RANGE_DAYS) throw new Error(`range longer than ${MAX_RANGE_DAYS} days`);
}
