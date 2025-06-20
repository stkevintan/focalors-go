FROM golang:1.24-alpine AS builder

# Declare TARGETARCH build argument which is automatically set by buildx
ARG TARGETARCH

WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application for the target architecture
# GOARCH will be amd64 or arm64 depending on the --platform flag used by docker buildx
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o focalors-go main.go

# Use a minimal base image
FROM alpine:latest

# Install timezone data
RUN apk --no-cache add tzdata

WORKDIR /app

# Copy the built binary from the builder stage
# This will be the binary for the specific architecture this stage is being built for
COPY --from=builder /app/focalors-go /app/focalors-go

# Set the entrypoint to run the application
CMD ["/app/focalors-go"]
