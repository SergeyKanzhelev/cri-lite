# Build stage
FROM golang:1.24.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN apk add --no-cache git

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X cri-lite/pkg/version.Version=$(git describe --tags --always --dirty)" -o /cri-lite .

# Final stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates

COPY --from=builder /cri-lite /cri-lite

ENTRYPOINT ["/cri-lite"]
