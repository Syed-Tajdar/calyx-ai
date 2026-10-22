import { Logger } from '@nestjs/common';
import { createTransport } from 'nodemailer';

/** Minimal transport surface (nodemailer's `sendMail`), so tests can pass a fake. */
export interface MailTransport {
  sendMail(msg: { from: string; to: string[]; subject: string; text: string }): Promise<unknown>;
}

export interface SmtpConfig {
  host: string;
  port: number;
  user: string;
  password: string;
  from: string;
  to: string[];
}

type Env = Record<string, string | undefined>;

/** SMTP settings from env, or null when email is off (SMTP_HOST empty or no recipient). */
export function smtpConfig(env: Env = process.env): SmtpConfig | null {
  const host = (env.SMTP_HOST ?? '').trim();
  if (!host) return null;
  const to = (env.ALERT_EMAIL_TO ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  if (to.length === 0) return null;
  const port = Number(env.SMTP_PORT || 587);
  return {
    host,
    port: Number.isFinite(port) && port > 0 ? port : 587,
    user: env.SMTP_USER ?? '',
    password: env.SMTP_PASSWORD ?? '',
    from: (env.SMTP_FROM ?? '').trim() || 'calyx-alerts@localhost',
    to,
  };
}

export const EMAIL_SENDER = 'WORKFLOW_EMAIL_SENDER';

export interface AlertEmail {
  subject: string;
  text: string;
}

/**
 * Alert email over SMTP (nodemailer). Off while SMTP_HOST is empty; the in-app bell works
 * without it. Logs only the outcome and the recipient count, never the password or addresses.
 */
export class EmailSender {
  private readonly logger = new Logger('WorkflowEmail');

  constructor(
    private readonly env: Env = process.env,
    private readonly transportFactory: (cfg: SmtpConfig) => MailTransport = (cfg) =>
      createTransport({
        host: cfg.host,
        port: cfg.port,
        secure: cfg.port === 465,
        auth: cfg.user ? { user: cfg.user, pass: cfg.password } : undefined,
      }),
  ) {}

  enabled(): boolean {
    return smtpConfig(this.env) !== null;
  }

  /** True when sent. Never throws. */
  async send(mail: AlertEmail): Promise<boolean> {
    const cfg = smtpConfig(this.env);
    if (!cfg) return false;
    try {
      await this.transportFactory(cfg).sendMail({ from: cfg.from, to: cfg.to, subject: mail.subject, text: mail.text });
      this.logger.log(`alert email sent to ${cfg.to.length} recipient(s)`);
      return true;
    } catch (err: unknown) {
      const code = (err as { code?: string })?.code ?? 'error';
      this.logger.warn(`alert email failed (${code}) for ${cfg.to.length} recipient(s)`);
      return false;
    }
  }
}
