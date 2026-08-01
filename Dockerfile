FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o grabba ./cmd/grabba

FROM alpine:3.19

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

COPY --from=builder /app/grabba /usr/local/bin/grabba

ENTRYPOINT ["grabba"]
CMD ["--help"]
