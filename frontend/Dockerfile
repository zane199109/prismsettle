# frontend Dockerfile — multi-stage build for the PrismSettle Next.js
# dashboard. Output image runs `next start` on Node 20-alpine.
#
# Build context: ./frontend
#   docker build -t prismsettle/frontend ./frontend

# ---------- Stage 1: deps ----------
FROM node:20-alpine AS deps

# libc6-compat is required by some Next.js native deps on Alpine.
RUN apk add --no-cache libc6-compat

WORKDIR /app

# Use China-accessible npm registry to avoid Docker build network timeouts.
RUN npm config set registry https://registry.npmmirror.com

# Copy lockfile + package.json first for layer caching.
COPY package.json package-lock.json* ./
RUN npm ci --no-audit --no-fund

# ---------- Stage 2: build ----------
FROM node:20-alpine AS builder

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Remove the dangling .env symlink (frontend/.env -> ../.env exists for
# local dev only; inside the image it breaks Next.js env loading with
# ENOENT during `next build`). NEXT_PUBLIC_* values come from .env.local
# (build) and docker-compose environment (runtime).
RUN rm -f .env

# Disable telemetry during build.
ENV NEXT_TELEMETRY_DISABLED=1

# Create public dir if missing (Next.js expects it, but project may not have static assets yet).
RUN mkdir -p public

# API URL is baked at build time; can be overridden at runtime via
# docker-compose environment. Empty default falls back to relative path
# which is proxied by Next.js rewrites in next.config.ts.
ARG NEXT_PUBLIC_API_BASE_URL=""
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL

RUN npm run build

# ---------- Stage 3: runtime ----------
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

# Non-root user.
RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001

# Copy standalone output (Next.js 15 output: "standalone") + static assets.
COPY --from=builder --chown=nextjs:nodejs /app/.next ./.next
COPY --from=builder --chown=nextjs:nodejs /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/package.json ./package.json
COPY --from=builder --chown=nextjs:nodejs /app/node_modules ./node_modules

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=20s --timeout=3s --retries=5 \
    CMD wget -qO- http://127.0.0.1:3000/ || exit 1

CMD ["npm", "start"]
