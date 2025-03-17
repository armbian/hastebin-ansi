FROM golang:1.24-alpine AS builder

WORKDIR /src

# Copy go mod and project files
COPY go.* ./
RUN go mod download
COPY . .

# Install necessary dependencies
RUN apk update && apk add --no-cache git ca-certificates openssl && update-ca-certificates

# Build the binary
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ./hastebin ./cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

# Copy the binary from the builder stage
COPY --from=builder /src/hastebin /bin/hastebin

# Set the user and group
USER 0:0

CMD ["/bin/hastebin", "--config", "/app/config.yaml"]