FROM node:24.12.0-bookworm-slim

WORKDIR /workspace
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .node-version ./
COPY apps/bff/package.json apps/bff/package.json
COPY apps/web/package.json apps/web/package.json
COPY packages/core-client/package.json packages/core-client/package.json
COPY packages/browser-client/package.json packages/browser-client/package.json
RUN pnpm install --frozen-lockfile

COPY apps/bff apps/bff
COPY packages/core-client packages/core-client
COPY packages/browser-client packages/browser-client
RUN pnpm --filter @fluctlight/core-client generate \
  && pnpm --filter @fluctlight/bff build

CMD ["node", "apps/bff/dist/server.js"]
