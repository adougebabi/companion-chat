FROM node:24.12.0-bookworm-slim AS build

WORKDIR /workspace
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .node-version ./
COPY apps/web/package.json apps/web/package.json
COPY packages/browser-client/package.json packages/browser-client/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web apps/web
COPY packages/browser-client packages/browser-client
RUN pnpm --filter @fluctlight/browser-client generate \
  && pnpm --filter @fluctlight/web build

FROM nginx:1.29-alpine

COPY --from=build /workspace/apps/web/dist /usr/share/nginx/html
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY apps/web/scripts/write-runtime-config.sh /docker-entrypoint.d/40-write-runtime-config.sh

RUN chmod +x /docker-entrypoint.d/40-write-runtime-config.sh

EXPOSE 80
