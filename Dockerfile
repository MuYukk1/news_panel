# Keep the production image to a single Go binary plus CA certificates.
FROM node:26-alpine AS frontend
WORKDIR /src/frontend
RUN npm install --global pnpm@10.34.9
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/index.html frontend/tsconfig*.json frontend/vite.config.ts ./
COPY frontend/src ./src
COPY frontend/public ./public
RUN pnpm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
COPY --from=frontend /src/frontend/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /news_panel ./cmd/fusion

FROM alpine:3.23
LABEL org.opencontainers.image.title="news_panel"
RUN apk add --no-cache ca-certificates && \
    addgroup -S news_panel && adduser -S -D -H -G news_panel news_panel && \
    mkdir -p /data && chown news_panel:news_panel /data
COPY --from=backend /news_panel /usr/local/bin/news_panel
COPY LICENSE NOTICE backend/THIRD_PARTY_NOTICES.txt /usr/share/licenses/news_panel/
USER news_panel
WORKDIR /data
VOLUME ["/data"]
ENV DB=/data/news_panel.db
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/oidc/enabled || exit 1
CMD ["news_panel"]
