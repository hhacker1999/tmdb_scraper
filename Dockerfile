# Stage 1: The "Heavy" Build Stage
FROM golang:1.23-alpine AS builder

# Install git if your go.mod has private repos or specific modules
RUN apk add --no-cache git

WORKDIR /app

# Only copy files needed for dependency resolution
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code (filtered by .dockerignore)
COPY . .

# Build a statically linked binary (CGO_ENABLED=0 is key for alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -o /scraper .

# Stage 2: The "Tiny" Final Stage
# This is the only part that gets "exported" as your final image
FROM alpine:3.19

WORKDIR /app

# Only copy the result from the builder
COPY --from=builder /scraper .

# Add CA certificates so the app can download from IMDb (HTTPS)
RUN apk --no-cache add ca-certificates

EXPOSE 6996

# Run the app
CMD ["./scraper"]
