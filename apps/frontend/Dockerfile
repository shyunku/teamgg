# syntax=docker/dockerfile:1

FROM node:20-bookworm-slim AS dependencies

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

FROM dependencies AS development

ENV NODE_ENV=development
COPY . .
EXPOSE 8080
CMD ["npm", "run", "dev"]

FROM dependencies AS builder

COPY . .
RUN npm run build

FROM nginx:1.27-alpine AS production

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/public /usr/share/nginx/html
# index.html selects the dev bundle on localhost. Mirror the production assets
# so the production image can also be smoke-tested through localhost.
RUN mkdir -p /usr/share/nginx/html/build/dev \
    && cp /usr/share/nginx/html/build/production/bundle.* /usr/share/nginx/html/build/dev/

EXPOSE 8080
