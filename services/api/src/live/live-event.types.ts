import { z } from 'zod';

export const liveEventTypeSchema = z.enum([
  'ping_received',
  'call_scored',
  'conversion_attributed',
  'compliance_violation',
]);

export type LiveEventType = z.infer<typeof liveEventTypeSchema>;

export const pingEventPayloadSchema = z.object({
  schema_version: z.literal('1'),
  event_id: z.string().min(1),
  tenant_id: z.string().uuid(),
  call_id: z.string().min(1),
  platform: z.string(),
  ani_hash: z.string().optional(),
  campaign_id: z.string().optional(),
  publisher_id: z.string().optional(),
  event_type: z.string().optional(),
  is_duplicate: z.boolean().optional(),
  received_at_unix_ms: z.number().int(),
});

export const callScoredPayloadSchema = z.object({
  schema_version: z.literal('1'),
  event_id: z.string().min(1),
  tenant_id: z.string().uuid(),
  call_id: z.string().min(1),
  overall_score: z.number(),
  sentiment: z.string().optional(),
  scored_at_unix_ms: z.number().int().optional(),
});

export const conversionAttributedPayloadSchema = z.object({
  tenant_id: z.string().uuid(),
  event_id: z.string().min(1),
  call_id: z.string().min(1),
  platform: z.string().max(32),
  click_id: z.string().max(256).optional(),
  revenue: z.number().optional(),
  status: z.enum(['fired', 'failed']),
  http_status: z.number().int().min(0).optional(),
  attributed_at: z.string(),
});

export const complianceViolationPayloadSchema = z.object({
  schema_version: z.literal('1'),
  event_id: z.string().min(1),
  tenant_id: z.string().uuid(),
  call_id: z.string().min(1),
  violation_type: z.string(),
  risk_score: z.number().optional(),
  exposure_usd: z.number().optional(),
  detected_at_unix_ms: z.number().int(),
});

export const liveEventEnvelopeSchema = z.object({
  type: liveEventTypeSchema,
  payload: z.union([
    pingEventPayloadSchema,
    callScoredPayloadSchema,
    conversionAttributedPayloadSchema,
    complianceViolationPayloadSchema,
  ]),
  emitted_at: z.string(),
});

export type LiveEventEnvelope = z.infer<typeof liveEventEnvelopeSchema>;

export type PingEventPayload = z.infer<typeof pingEventPayloadSchema>;
export type CallScoredPayload = z.infer<typeof callScoredPayloadSchema>;
export type ConversionAttributedPayload = z.infer<typeof conversionAttributedPayloadSchema>;
export type ComplianceViolationPayload = z.infer<typeof complianceViolationPayloadSchema>;

export function tenantRoom(tenantId: string): string {
  return `tenant:${tenantId}`;
}
