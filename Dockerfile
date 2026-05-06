FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/bot ./cmd/bot && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/app ./cmd/app

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /bin/bot /app/bot
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /bin/app /app/app
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/migrations /app/migrations
RUN mkdir -p /app/static
COPY --from=builder /app/static /app/static

EXPOSE 8080 8081

CMD ["/app/bot"]
