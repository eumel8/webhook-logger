FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o webhook-logger .

FROM scratch
COPY --from=builder /app/webhook-logger /webhook-logger
EXPOSE 9095
ENTRYPOINT ["/webhook-logger"]
