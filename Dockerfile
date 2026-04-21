FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod ./

# Download dependencies
ENV GOFLAGS=-mod=mod
RUN go mod download

# Copy source code
COPY backend/main.go ./main.go

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -a -installsuffix cgo -o main .

# Minimal final image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]