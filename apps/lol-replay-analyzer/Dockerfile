# syntax=docker/dockerfile:1

FROM node:24-bookworm-slim AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

COPY tsconfig.json ./
COPY src ./src
COPY test ./test
RUN npm run build \
    && npm prune --omit=dev

FROM node:24-bookworm-slim AS runtime

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=7720 \
    WORK_DIR=/app/.work \
    ROFL_CACHE_DIR=/app/.rofl-cache

WORKDIR /app
COPY --from=builder --chown=node:node /app/package.json /app/package-lock.json ./
COPY --from=builder --chown=node:node /app/node_modules ./node_modules
COPY --from=builder --chown=node:node /app/dist/src ./dist/src
RUN mkdir -p /app/.work /app/.rofl-cache \
    && chown -R node:node /app

USER node
VOLUME ["/app/.rofl-cache"]
EXPOSE 7720

CMD ["node", "dist/src/server.js"]
