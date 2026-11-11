FROM node:26.13.0-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.4-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/webui/assets/dist/ internal/webui/assets/dist/
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata -ldflags="-s -w" -o /out/fts ./cmd/fts \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/playwright github.com/mxschmitt/playwright-go/cmd/playwright

FROM debian:trixie-20261104-slim
ENV PLAYWRIGHT_DRIVER_PATH=/opt/playwright/driver \
    PLAYWRIGHT_BROWSERS_PATH=/opt/playwright/browsers
COPY --from=build /out/playwright /usr/local/bin/playwright-installer
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates tzdata supervisor matchbox-window-manager \
        tigervnc-standalone-server novnc python3-websockify \
        fonts-dejavu-core fonts-noto-core \
    && playwright-installer install --with-deps --no-shell chromium \
    && rm /usr/local/bin/playwright-installer \
    && chmod -R a+rX /opt/playwright \
    && rm -rf /var/lib/apt/lists/* /root/.cache \
    && groupadd --gid 1000 app \
    && useradd --uid 1000 --gid 1000 --create-home --shell /usr/sbin/nologin app \
    && mkdir -p /data \
    && chown 1000:1000 /data
COPY --from=build /out/fts /usr/local/bin/fts
COPY deploy/supervisord.conf /etc/supervisord.conf
USER 1000:1000
ENV HOME=/home/app \
    DISPLAY=:1 \
    FTS_ADDR=0.0.0.0:8787 \
    FTS_LISTEN_ANY=true \
    FTS_LOGIN_MODE=window \
    FTS_VIEWER_ADDR=127.0.0.1:6080 \
    FTS_BROWSER_CHANNEL=chromium \
    FTS_DATA_DIR=/data \
    FTS_SECRET_KEY_FILE=/run/secrets/fts_key
EXPOSE 8787
ENTRYPOINT ["/usr/bin/supervisord", "-n", "-c", "/etc/supervisord.conf"]
