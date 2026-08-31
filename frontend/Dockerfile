# =============================================================================
# Frontend - Multi-stage Dockerfile
# =============================================================================
# Production: Static export served by nginx (see docker-compose.ecs.yml)
# Development: Next.js dev server with hot reload
# =============================================================================

# -----------------------------------------------------------------------------
# Development Stage
# -----------------------------------------------------------------------------
FROM node:20-alpine AS development

WORKDIR /app

# Install dependencies
COPY package.json package-lock.json* ./
RUN npm install

# Copy source code
COPY . .

# Expose port
EXPOSE 3000

# Environment
ENV NODE_ENV=development
ENV NEXT_TELEMETRY_DISABLED=1

# Run development server
CMD ["npm", "run", "dev"]

# -----------------------------------------------------------------------------
# Dependencies Stage
# -----------------------------------------------------------------------------
FROM node:20-alpine AS deps

WORKDIR /app

# Install dependencies only
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

# -----------------------------------------------------------------------------
# Builder Stage (Static Export)
# -----------------------------------------------------------------------------
FROM node:20-alpine AS builder

WORKDIR /app

# Copy dependencies
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Build arguments for environment variables (baked into static build)
ARG NEXT_PUBLIC_API_URL=https://api.irislondonshoes.com
ARG NEXT_PUBLIC_AI_API_URL=https://api.irislondonshoes.com

ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_AI_API_URL=${NEXT_PUBLIC_AI_API_URL}
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

# Build static export to out/ directory
RUN npm run build

# -----------------------------------------------------------------------------
# Production Stage (Nginx Static Server)
# -----------------------------------------------------------------------------
# For local/Docker testing only - production uses host nginx
FROM nginx:alpine AS production

# Copy static files to nginx
COPY --from=builder /app/out /usr/share/nginx/html

# Copy nginx config for SPA routing
RUN echo 'server { \
    listen 80; \
    root /usr/share/nginx/html; \
    index index.html; \
    location / { \
        try_files $uri $uri/ $uri.html /index.html; \
    } \
    location /_next/static/ { \
        expires 1y; \
        add_header Cache-Control "public, immutable"; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost/ || exit 1

CMD ["nginx", "-g", "daemon off;"]

# -----------------------------------------------------------------------------
# Export Stage (for CI/CD - outputs static files only)
# -----------------------------------------------------------------------------
FROM scratch AS export

# Copy only the static files for extraction
COPY --from=builder /app/out /out
