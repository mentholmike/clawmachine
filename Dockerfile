# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o clawmachine ./cmd/clawmachine/

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /build/clawmachine /usr/local/bin/clawmachine

ENTRYPOINT ["clawmachine"]