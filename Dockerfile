# Build the web UI first; its dist is copied into the final image.
FROM node:24-alpine AS ui
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build the server binary (pure Go: no cgo, no libc dependency).
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd cmd
COPY internal internal
RUN CGO_ENABLED=0 go build -o /out/ledger ./cmd/ledger

FROM alpine:3.21
COPY --from=build /out/ledger /usr/local/bin/ledger
COPY --from=ui /src/web/dist /app/web/dist
WORKDIR /app

# SQLite data lives on this volume; survives container restarts.
VOLUME /data
ENV LEDGER_DB=/data/ledger.db
EXPOSE 8080

ENTRYPOINT ["ledger", "serve", "-addr", ":8080", "-ui", "web/dist"]
