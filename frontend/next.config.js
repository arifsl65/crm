/** @type {import('next').NextConfig} */
const nextConfig = {
  // Static export for nginx serving (no Node.js server required)
  // Dynamic routes like /dashboard/clients/[id] work client-side with
  // fallback pages. Builds to out/ directory.
  output: 'export',

  // Trailing slashes for static hosting compatibility
  trailingSlash: true,

  // Disable image optimization (no external service)
  images: {
    unoptimized: true,
  },

  // Environment variables exposed to the browser
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080',
    NEXT_PUBLIC_AI_API_URL: process.env.NEXT_PUBLIC_AI_API_URL || 'http://localhost:8000',
  },

  // Strict mode for development
  reactStrictMode: true,

  // Disable x-powered-by header
  poweredByHeader: false,

  // Compiler options
  compiler: {
    // Remove console.log in production
    removeConsole: process.env.NODE_ENV === 'production',
  },

  // Note: Security headers are configured in nginx (see nginx/conf.d/crm.conf)
  // headers() function doesn't work with static export
};

module.exports = nextConfig;
