FROM node:22.23.2-bookworm-slim AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN apt-get update && apt-get install -y --no-install-recommends python3 make g++ && rm -rf /var/lib/apt/lists/* && npm ci --omit=dev

FROM node:22.23.2-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production \
    PORT=4178 \
    DATA_DIR=/app/data
COPY --from=build /app/node_modules ./node_modules
COPY package.json ./
COPY server ./server
COPY src ./src
RUN mkdir -p /app/data && chown -R node:node /app
USER node
EXPOSE 4178
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD node -e "fetch('http://127.0.0.1:4178/api/health').then(r => process.exit(r.ok ? 0 : 1)).catch(() => process.exit(1))"
CMD ["npm", "start"]
