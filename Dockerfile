FROM golang:1.22-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mirror ./cmd/mirror

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/mirror /app/mirror
COPY web /app/web
COPY docs /app/docs
COPY scripts /app/scripts
ENV WEB_ROOT=/app/web \
    DOCS_ROOT=/app/docs \
    SCRIPTS_ROOT=/app/scripts \
    LISTEN=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
CMD ["/app/mirror"]
