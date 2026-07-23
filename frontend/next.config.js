/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Standalone output for optimized Docker images (ui-ux-pro-max Next.js guideline).
  output: "standalone",
  // Proxy API requests to the Go offchain service to avoid CORS during dev.
  // In production, the reverse proxy (nginx/ingress) handles this.
  async rewrites() {
    const backend = process.env.BACKEND_URL || "http://localhost:9527";
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
    ];
  },
};

module.exports = nextConfig;
