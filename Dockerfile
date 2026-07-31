FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download Go modules
COPY go.mod ./
RUN go mod tidy

# Copy the source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /aegisproxy ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /

COPY --from=builder /aegisproxy /aegisproxy

EXPOSE 8080 9090

ENTRYPOINT ["/aegisproxy"]
