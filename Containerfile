FROM docker.io/library/golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev sqlite-dev git

COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o nydus ./cmd/nydus

FROM docker.io/library/alpine:3.23

RUN apk add --no-cache ca-certificates sqlite-libs bash

WORKDIR /app
COPY --from=builder /app/nydus .

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/tmp/nydus.db
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

EXPOSE 15318

CMD ["./nydus"]