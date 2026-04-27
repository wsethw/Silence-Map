# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/silence-map ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 appuser

COPY --from=build /out/silence-map /app/silence-map
COPY web /app/web

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/silence-map"]
