FROM golang:1.26-alpine AS build

WORKDIR /src
COPY apps/gateway-go/go.mod apps/gateway-go/go.mod
COPY apps/gateway-go apps/gateway-go
WORKDIR /src/apps/gateway-go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fluctlight-bff ./cmd/gateway

FROM alpine:3.22

RUN apk add --no-cache curl \
  && addgroup -S fluctlight \
  && adduser -S -G fluctlight fluctlight
COPY --from=build /out/fluctlight-bff /usr/local/bin/fluctlight-bff

USER fluctlight
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/fluctlight-bff"]
