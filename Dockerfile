# --- Build Stage ---
FROM golang:1.23-alpine AS builder

# Install git and certificates (required for fetching dependencies)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependency files first to leverage Docker layer caching
COPY go.mod ./
# If go.sum doesn't exist yet, Docker will fail here. 
# --- Build Stage ---
# Use standard Golang image (based on Debian) instead of Alpine
FROM golang:1.23 AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod ./
# Copy go.sum if it exists, otherwise ignore
COPY go.sum* ./

# Download dependencies
# (Debian handles DNS/Certificates automatically, no extra installation needed)
RUN go mod download

# Copy source
COPY . .

# Build the application
# CGO_ENABLED=0 creates a static binary that works anywhere
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# --- Final Stage ---
# Use Debian Bookworm Slim (Small, secure, but has stable DNS)
FROM debian:bookworm-slim

# Install CA certificates for HTTPS requests (e.g. TMDB API)
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

CMD ["./main"]# Using a wildcard 'go.sum*' ensures it copies if it exists, but doesn't crash if it doesn't.
