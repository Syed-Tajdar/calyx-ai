import { createCipheriv, createDecipheriv, randomBytes } from 'crypto';

const NONCE_SIZE = 12;

function getEncryptionKey(): Buffer {
  const hex = process.env.CREDENTIAL_ENCRYPTION_KEY;
  if (!hex || hex.length !== 64) {
    throw new Error(
      'CREDENTIAL_ENCRYPTION_KEY must be 64 hex chars (32 bytes)',
    );
  }
  return Buffer.from(hex, 'hex');
}

export function encryptCredentialJson(payload: Record<string, string>): Buffer {
  const key = getEncryptionKey();
  const nonce = randomBytes(NONCE_SIZE);
  const cipher = createCipheriv('aes-256-gcm', key, nonce);
  const plain = Buffer.from(JSON.stringify(payload), 'utf8');
  const encrypted = Buffer.concat([cipher.update(plain), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([nonce, encrypted, tag]);
}

export function decryptCredentialJson(blob: Buffer): Record<string, string> {
  const key = getEncryptionKey();
  if (blob.length < NONCE_SIZE + 16) {
    throw new Error('credential blob too short');
  }
  const nonce = blob.subarray(0, NONCE_SIZE);
  const tag = blob.subarray(blob.length - 16);
  const ciphertext = blob.subarray(NONCE_SIZE, blob.length - 16);
  const decipher = createDecipheriv('aes-256-gcm', key, nonce);
  decipher.setAuthTag(tag);
  const plain = Buffer.concat([decipher.update(ciphertext), decipher.final()]);
  return JSON.parse(plain.toString('utf8')) as Record<string, string>;
}
