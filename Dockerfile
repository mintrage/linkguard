FROM golang:1.26.1-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG APP_PATH

RUN go build -o microservice ./${APP_PATH}

FROM alpine:latest
WORKDIR /root/

COPY --from=builder /app/microservice .

CMD ["./microservice"]