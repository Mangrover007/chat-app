# Stage 1: Build the binary
FROM golang:1.26.1-alpine as builder

# Copy and install dependencies so that it is cached
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o chat-service ./cmd/server/

# Stage 2: Run the binary
FROM scratch

ENV DB_URL="postgres://postgres:mango@postgres-service/discordv2"
ENV REDIS_URL="redis-service:6379"
ENV REDIS_PASSWORD=""

ENV SERVER_PORT=8080

WORKDIR /app
COPY --from=builder /app/chat-service .

EXPOSE 8080
CMD ["./chat-service"]
