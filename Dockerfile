FROM node:26.13.0-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.4-alpine3.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/webui/assets/dist/ internal/webui/assets/dist/
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata -ldflags="-s -w" -o /out/fts ./cmd/fts \
    && mkdir -p /out/data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/fts /fts
COPY --from=build --chown=1000:1000 /out/data /data
USER 1000:1000
ENV FTS_ADDR=0.0.0.0:8787 \
    FTS_LISTEN_ANY=true \
    FTS_LOGIN_MODE=bookmarklet \
    FTS_DATA_DIR=/data \
    FTS_SECRET_KEY_FILE=/run/secrets/fts_key
EXPOSE 8787
ENTRYPOINT ["/fts"]
