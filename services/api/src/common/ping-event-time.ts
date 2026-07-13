/** Prefer platform source call time; fall back to ingestion time. */
export const EVENT_AT_EXPR =
  "if(occurred_at > toDateTime(0), occurred_at, received_at)";
