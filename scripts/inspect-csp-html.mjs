import fs from 'fs';

const hdrPath = process.argv[2];
const bodyPath = process.argv[3];
const hdr = fs.readFileSync(hdrPath, 'utf8');
const h = fs.readFileSync(bodyPath, 'utf8');
const inline = [...h.matchAll(/<script(?![^>]*\ssrc=)[^>]*>/g)].length;
const nonce = [...h.matchAll(/<script[^>]*nonce=/g)].length;
const sample = (h.match(/<script[^>]*nonce="[^"]+"[^>]*>/) || [''])[0].slice(0, 160);
const csp = (hdr.match(/content-security-policy:[^\n]+/i) || [''])[0];
const cache = (hdr.match(/cache-control:[^\n]+/i) || [''])[0];
const nextCache = (hdr.match(/x-nextjs-cache:[^\n]+/i) || [''])[0];
const status = (hdr.match(/^HTTP\/[^\n]+/m) || [''])[0];
console.log(JSON.stringify({ status, cache, nextCache, csp: csp.slice(0, 200), inline, nonce, sample }, null, 2));
