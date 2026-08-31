FROM golang:1.25.4-alpine AS build

WORKDIR /src
COPY apps/core-go/go.mod apps/core-go/go.sum* ./apps/core-go/
COPY apps/core-go ./apps/core-go
WORKDIR /src/apps/core-go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-core-go ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-worker-go ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-migrate-go ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-cutover-go ./cmd/cutover
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-setup-token-go ./cmd/setup-token

FROM alpine:3.22

RUN apk add --no-cache wget tzdata \
  && addgroup -S fluctlight \
  && adduser -S -G fluctlight fluctlight
COPY --from=build /out/fluctlight-core-go /usr/local/bin/fluctlight-core-go
COPY --from=build /out/fluctlight-worker-go /usr/local/bin/fluctlight-worker-go
COPY --from=build /out/fluctlight-migrate-go /usr/local/bin/fluctlight-migrate-go
COPY --from=build /out/fluctlight-cutover-go /usr/local/bin/fluctlight-cutover-go
COPY --from=build /out/fluctlight-setup-token-go /usr/local/bin/fluctlight-setup-token-go

USER fluctlight
EXPOSE 8080
CMD ["/usr/local/bin/fluctlight-core-go"]
