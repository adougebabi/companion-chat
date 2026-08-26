FROM node:24.12.0-bookworm-slim

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

EXPOSE 4173
CMD ["node", "apps/web/scripts/serve-preview.mjs"]
