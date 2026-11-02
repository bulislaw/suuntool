# syntax=docker/dockerfile:1
FROM golang:1.26.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/suuntool .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 suuntool \
    && adduser -D -u 10001 -G suuntool suuntool \
    && mkdir -p /data/config /data/cache \
    && chown -R suuntool:suuntool /data
COPY --from=build /out/suuntool /usr/local/bin/suuntool
ENV HOME=/home/suuntool \
    XDG_CONFIG_HOME=/data/config \
    XDG_CACHE_HOME=/data/cache
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["suuntool"]
CMD ["mcp", "--listen", ":8080", "--verbose"]
