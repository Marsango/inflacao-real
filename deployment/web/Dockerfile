FROM node:22-bookworm-slim@sha256:4b150b48b4116e2a22f2c39cc27025fbf4f153e2078fd3b14d159fdb5c7279da AS dependencies
WORKDIR /source/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
COPY docs/contracts/web/ /source/docs/contracts/web/
FROM dependencies AS checks
RUN npm run typecheck && npm test
FROM dependencies AS build
RUN npm run build
FROM caddy:2-alpine@sha256:e70b164da81ef698b3ad19af799ea6086ce6d9e48ed4079eb9fe26f6fbf14ee5
RUN mkdir -p /srv /data/caddy /config/caddy && chown -R 10001:10001 /srv /data /config
COPY --from=build --chown=10001:10001 /source/web/dist/ /srv/
COPY deployment/web/Caddyfile /etc/caddy/Caddyfile
USER 10001:10001
