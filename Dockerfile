# syntax=docker/dockerfile:1

FROM node:22-alpine AS web-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web-build /src/web/dist ./web/dist
RUN go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server

FROM alpine:3.23
LABEL org.opencontainers.image.source="https://github.com/skobkin/meshmap-lite"
WORKDIR /app
RUN addgroup -S app && adduser -S app -G app
COPY --from=go-build /out/server /usr/local/bin/server
COPY --from=web-build /src/web/dist ./web/dist
EXPOSE 8080
USER app
ENTRYPOINT ["server"]
