FROM golang:1.26-bookworm AS builder

WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build with CGO enabled (required for ONNX Runtime / Hugot)
RUN CGO_ENABLED=1 GOOS=linux go build -o /aegisproxy ./cmd/server

# Final stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /

COPY --from=builder /aegisproxy /aegisproxy

EXPOSE 8080 9090

ENTRYPOINT ["/aegisproxy"]
