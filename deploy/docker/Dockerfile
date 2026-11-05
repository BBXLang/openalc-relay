# syntax=docker/dockerfile:1
FROM golang:1.26.10-bookworm AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/relay ./cmd/relay
RUN mkdir -p /out/state && chmod 0700 /out/state

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/relay /relay
COPY --from=build --chown=65532:65532 /out/state /var/lib/relay
USER 65532:65532
WORKDIR /var/lib/relay
ENV RELAY_HTTP_ADDR=0.0.0.0:8080
ENV RELAY_DATABASE_PATH=/var/lib/relay/relay.db
ENV RELAY_LOG_LEVEL=info
EXPOSE 8080
ENTRYPOINT ["/relay"]
