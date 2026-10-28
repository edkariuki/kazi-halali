# Kazi Halali API
#
# Build from the REPO ROOT (the image also needs data/):
#   docker build -f api/Dockerfile -t kazi-halali-api .
#
# Run:
#   docker run -p 8080:8080 --env-file .env -e STORE=memory kazi-halali-api
#
# The image contains data/ as it is on disk when you build. If data/real/
# exists (the NEA register export), it is included; set
#   AGENCIES_CSV=/app/data/real/agencies.csv
# to use it. Secrets (.env) are never copied in — see .dockerignore.

# ---- build ---------------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

# Dependencies first, so they are cached between builds.
COPY api/go.mod api/go.sum* ./
RUN go mod download

COPY api/ ./
# go mod tidy is a no-op when go.mod/go.sum are already complete.
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- run -----------------------------------------------------------------
# distroless/static: no shell, runs as non-root, includes CA certificates
# (needed for HTTPS calls to Token Factory and Tavily).
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY data/ /app/data/

ENV PORT=8080 \
    STORE=memory \
    AGENCIES_CSV=/app/data/sample/agencies.csv \
    FLAGGED_CSV=/app/data/sample/flagged.csv

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]