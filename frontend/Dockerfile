# =============================================================================
# Frontend - Multi-stage Dockerfile
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
RUN npm ci --only=production

# -----------------------------------------------------------------------------
# Builder Stage
# -----------------------------------------------------------------------------
FROM node:20-alpine AS builder

WORKDIR /app

# Copy dependencies
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Build arguments for environment variables
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_AI_API_URL

ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_AI_API_URL=${NEXT_PUBLIC_AI_API_URL}
ENV NEXT_TELEMETRY_DISABLED=1
ENV NODE_ENV=production

# Build the application
RUN npm run build

# -----------------------------------------------------------------------------
# Production Stage (Static Export)
# -----------------------------------------------------------------------------
FROM nginx:alpine AS production

# Copy custom nginx config
COPY --from=builder /app/nginx.conf /etc/nginx/nginx.conf

# Copy static files from builder
COPY --from=builder /app/out /usr/share/nginx/html

# Security: Remove default nginx welcome page
RUN rm -rf /usr/share/nginx/html/index.html.default 2>/dev/null || true

# Expose port
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost/health || exit 1

# Run nginx
CMD ["nginx", "-g", "daemon off;"]
