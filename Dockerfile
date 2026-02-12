# Build stage
FROM golang:1.25 AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o music-dl ./cmd/music-dl

# Runtime stage
FROM alpine:latest

# [新增] 替换为阿里云镜像源，解决 TLS 连接错误和速度慢的问题
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create a non-root user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /home/appuser/

# Copy the binary from builder stage
COPY --from=builder /app/music-dl .

# Create downloads directory
RUN mkdir -p downloads

# Change ownership to non-root user
RUN chown -R appuser:appuser /home/appuser/

# Switch to non-root user
USER appuser

# Expose port 8080
EXPOSE 8080

# Run the web server by default
CMD ["./music-dl", "web", "--port", "8080"]