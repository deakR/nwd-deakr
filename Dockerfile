# ---- Build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads (stdlib-only today; future-proof)
COPY go.mod ./
RUN go mod download

# Repo root is the build context: main.go, docs.go, internal/, openapi.yaml,
# assets/ (openapi.yaml and assets/ are embedded via go:embed).
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/nwd-deakr .

# ---- Runtime stage ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 app
USER app
COPY --from=build /out/nwd-deakr /usr/local/bin/nwd-deakr
EXPOSE 8080
ENTRYPOINT ["nwd-deakr"]
