FROM golang:1.26-alpine AS build

WORKDIR /src
COPY apps/core-go/go.mod apps/core-go/go.sum* ./apps/core-go/
COPY apps/core-go ./apps/core-go
WORKDIR /src/apps/core-go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-core-go ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache wget \
  && addgroup -S fluctlight \
  && adduser -S -G fluctlight fluctlight
COPY --from=build /out/fluctlight-core-go /usr/local/bin/fluctlight-core-go

USER fluctlight
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/fluctlight-core-go"]
