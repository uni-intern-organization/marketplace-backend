FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /marketplace-api ./cmd/api

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /marketplace-api .
COPY migrations ./migrations

EXPOSE 8081

CMD ["./marketplace-api"]
