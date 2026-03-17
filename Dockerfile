FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o draft-thinker ./cmd/gateway

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /app/draft-thinker /usr/local/bin/draft-thinker
COPY --from=builder /app/config.yaml /etc/draft-thinker/config.yaml

EXPOSE 8080

ENTRYPOINT ["draft-thinker"]
CMD ["--config", "/etc/draft-thinker/config.yaml"]
