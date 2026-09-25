/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Prefer nonces from middleware over blanket unsafe-inline for scripts.
  experimental: {
    // Next 14: allow reading nonce from middleware headers in App Router.
  },
  poweredByHeader: false,
};

export default nextConfig;
