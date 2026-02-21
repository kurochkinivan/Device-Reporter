FROM golang:alpine3.23 AS builder

ARG APP_VERSION=1.0.0

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0" -o /bin/device_reporter ./cmd/device_reporter
RUN CGO_ENABLED=0 go build -o /bin/migrate ./cmd/migrator

FROM alpine:3.23

RUN apk --no-cache add tzdata

WORKDIR /app

COPY --from=builder /bin/device_reporter /bin/device_reporter
COPY --from=builder /bin/migrate /bin/migrate

CMD ["/bin/device_reporter"]

