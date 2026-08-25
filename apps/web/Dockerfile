FROM node:24.12.0-bookworm-slim

WORKDIR /workspace
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml .node-version ./
COPY apps/web/package.json apps/web/package.json
COPY packages/browser-client/package.json packages/browser-client/package.json
RUN pnpm install --frozen-lockfile

COPY apps/web apps/web
COPY packages/browser-client packages/browser-client
ARG VITE_BFF_ORIGIN=http://127.0.0.1:13000
ENV VITE_BFF_ORIGIN=$VITE_BFF_ORIGIN
RUN pnpm --filter @fluctlight/browser-client generate \
  && pnpm --filter @fluctlight/web build

EXPOSE 4173
CMD ["pnpm", "--filter", "@fluctlight/web", "exec", "vite", "preview", "--host", "0.0.0.0", "--port", "4173"]
