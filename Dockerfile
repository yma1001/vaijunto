# syntax=docker/dockerfile:1
# Imagem única: o binário é escolhido por BUILD_TARGET (server|driver|passenger|loadtest).
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ARG BUILD_TARGET=server
RUN CGO_ENABLED=0 go build -o /out/vaijunto ./cmd/${BUILD_TARGET}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/vaijunto /usr/local/bin/vaijunto
ENV SERVER_HOST=127.0.0.1
ENV SERVER_PORT=5000
ENV LISTEN_HOST=0.0.0.0
ENV DATA_PATH=/data/state.json
VOLUME ["/data"]
EXPOSE 5000
ENTRYPOINT ["/usr/local/bin/vaijunto"]
