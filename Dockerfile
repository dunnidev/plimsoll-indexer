# syntax=docker/dockerfile:1
# Multi-stage build: compile with the go.mod toolchain, ship a distroless static image.
FROM golang:1.27-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/plimsoll-indexer ./cmd/plimsoll-indexer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/plimsoll-indexer /plimsoll-indexer
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/plimsoll-indexer"]
