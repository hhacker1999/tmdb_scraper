# --- Build Stage ---
FROM golang:1.23-alpine AS builder

# Install git and certificates (required for fetching dependencies)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy dependency files first to leverage Docker layer caching
COPY go.mod ./
# If go.sum doesn't exist yet, Docker will fail here. 
# Using a wildcard 'go.sum*' ensures it copies if it exists, but doesn't crash if it doesn't.
COPY go.sum* ./

# Download dependencies
RUN go mod download

# Now copy the rest of the source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# --- Final Stage ---
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .

CMD ["./main"]
