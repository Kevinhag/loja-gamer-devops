# syntax=docker/dockerfile:1

# ---- 1. Frontend (Svelte + Vite) ----
FROM node:24-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 2. Backend (Go) ----
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG APP_VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${APP_VERSION}" -o /out/server ./cmd/server \
 && mkdir -p /out/data/images

# ---- 3. Imagem final: só o binário, o frontend compilado e as imagens iniciais ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /web/dist /app/public
COPY seed-images /app/seed-images
COPY --from=api --chown=nonroot:nonroot /out/data /data

ENV PORT=8080 \
    STATIC_DIR=/app/public \
    SEED_IMAGES_DIR=/app/seed-images \
    IMAGES_DIR=/data/images

EXPOSE 8080
USER nonroot
HEALTHCHECK --interval=15s --timeout=3s --start-period=30s --retries=3 CMD ["/app/server", "-healthcheck"]
ENTRYPOINT ["/app/server"]
