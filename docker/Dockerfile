# Frontend build stage
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy frontend dependency files first for caching
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Copy frontend source and build
COPY frontend/ .
RUN npm run build

# Go build stage
FROM golang:1.24.5-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Embed frontend build into Go binary
COPY --from=frontend-builder /app/frontend/dist/ ./internal/frontend/dist/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o whatomate ./cmd/whatomate

# Piper TTS download stage
FROM debian:bookworm-slim AS piper-dl
RUN apt-get update && apt-get install -y --no-install-recommends wget ca-certificates && rm -rf /var/lib/apt/lists/*
RUN wget -q https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_x86_64.tar.gz -O /tmp/piper.tar.gz \
    && tar xf /tmp/piper.tar.gz -C /tmp \
    && rm /tmp/piper.tar.gz
# Download default English voice model (~60MB)
RUN mkdir -p /tmp/piper-models \
    && wget -q https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx \
       -O /tmp/piper-models/en_US-lessac-medium.onnx \
    && wget -q https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/lessac/medium/en_US-lessac-medium.onnx.json \
       -O /tmp/piper-models/en_US-lessac-medium.onnx.json

# Final stage (Debian for glibc — required by Piper TTS)
FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies (TTS: espeak-ng, opus-tools; transcoding: ffmpeg)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata espeak-ng opus-tools ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /app/whatomate .

# Copy config example (will be overridden by env vars in production)
COPY --from=builder /app/config.example.toml ./config.toml


# Install Piper TTS binary + shared libraries
COPY --from=piper-dl /tmp/piper/piper /usr/local/bin/piper
COPY --from=piper-dl /tmp/piper/lib*.so* /usr/local/lib/
COPY --from=piper-dl /tmp/piper/espeak-ng-data /usr/share/espeak-ng-data
RUN ldconfig

# Install default voice model
COPY --from=piper-dl /tmp/piper-models /opt/piper/models

# Create directories
RUN mkdir -p /app/uploads /app/audio

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./whatomate", "server", "-config", "config.toml", "-migrate"]
