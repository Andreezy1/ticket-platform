FROM golang:1.26.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o ticket-platform-event ./cmd/event

FROM alpine:3.20

WORKDIR /app

RUN adduser -D appuser
USER appuser

COPY --from=builder /app/ticket-platform-event .

EXPOSE 8081

CMD ["./ticket-platform-event"]