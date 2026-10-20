/**
 * Minimal OpenAI Responses API client (no SDK dependency). Used by Ask AI only.
 *
 * - `store: false` on every request (owner decision Q8): OpenAI keeps no application state.
 * - With `store: false`, earlier output items are sent back in `input` on the next turn; for
 *   reasoning models their encrypted reasoning is requested so the turn can continue.
 * - The API key is sent only in the Authorization header and never appears in an error.
 */

export interface ResponsesUsage {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
}

export type ResponseItem = Record<string, unknown> & { type: string };

export interface ResponsesResult {
  output: ResponseItem[];
  usage: ResponsesUsage;
}

export interface ResponsesRequest {
  instructions: string;
  input: ResponseItem[];
  tools: readonly unknown[];
  textFormat: unknown;
  maxOutputTokens: number;
}

export interface ResponsesClient {
  create(req: ResponsesRequest): Promise<ResponsesResult>;
}

export class OpenAIError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

const KEY_RE = /sk-[A-Za-z0-9_-]{8,}/g;

export function scrubKey(text: string): string {
  return text.replace(KEY_RE, 'sk-[REDACTED]');
}

export function isReasoningModel(model: string): boolean {
  return /^(o\d|gpt-5|gpt-6)/.test(model);
}

export class OpenAIResponsesClient implements ResponsesClient {
  constructor(
    private readonly apiKey: string,
    private readonly model: string,
    private readonly baseUrl = 'https://api.openai.com',
    private readonly timeoutMs = 45_000,
    private readonly fetchImpl: typeof fetch = fetch,
  ) {}

  async create(req: ResponsesRequest): Promise<ResponsesResult> {
    const body: Record<string, unknown> = {
      model: this.model,
      instructions: req.instructions,
      input: req.input,
      tools: req.tools,
      tool_choice: 'auto',
      parallel_tool_calls: true,
      text: { format: req.textFormat },
      store: false,
      max_output_tokens: req.maxOutputTokens,
    };
    if (isReasoningModel(this.model)) body.include = ['reasoning.encrypted_content'];

    for (let attempt = 1; attempt <= 2; attempt++) {
      const ctrl = new AbortController();
      const timer = setTimeout(() => ctrl.abort(), this.timeoutMs);
      let res: Response;
      try {
        res = await this.fetchImpl(`${this.baseUrl.replace(/\/$/, '')}/v1/responses`, {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${this.apiKey}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(body),
          signal: ctrl.signal,
        });
      } catch (err) {
        clearTimeout(timer);
        if (attempt === 2) {
          const kind = err instanceof Error && err.name === 'AbortError' ? 'timeout' : 'network error';
          throw new OpenAIError(`OpenAI request failed: ${kind}`, 502);
        }
        continue;
      }
      clearTimeout(timer);
      if ((res.status === 429 || res.status >= 500) && attempt === 1) {
        await new Promise((r) => setTimeout(r, 1000));
        continue;
      }
      const text = await res.text();
      if (!res.ok) {
        let msg = `HTTP ${res.status}`;
        try {
          const parsed = JSON.parse(text) as { error?: { message?: string; code?: string } };
          if (parsed.error?.message) msg += `: ${parsed.error.message}`;
        } catch {
          /* non-JSON error body: keep the status only */
        }
        throw new OpenAIError(scrubKey(`OpenAI ${msg}`).slice(0, 300), res.status);
      }
      const parsed = JSON.parse(text) as {
        output?: ResponseItem[];
        usage?: Partial<ResponsesUsage>;
        status?: string;
        incomplete_details?: { reason?: string };
      };
      if (parsed.status && parsed.status !== 'completed') {
        throw new OpenAIError(
          `OpenAI response ${parsed.status}${parsed.incomplete_details?.reason ? `: ${parsed.incomplete_details.reason}` : ''}`,
          502,
        );
      }
      const u = parsed.usage ?? {};
      return {
        output: parsed.output ?? [],
        usage: {
          input_tokens: Number(u.input_tokens ?? 0),
          output_tokens: Number(u.output_tokens ?? 0),
          total_tokens: Number(u.total_tokens ?? Number(u.input_tokens ?? 0) + Number(u.output_tokens ?? 0)),
        },
      };
    }
    throw new OpenAIError('OpenAI request failed', 502);
  }
}
