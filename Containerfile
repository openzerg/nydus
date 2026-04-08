FROM docker.io/library/golang:1.25-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y gcc libc6-dev libsqlite3-dev git && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o nydus ./cmd/nydus

FROM docker.io/library/busybox:glibc

WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/nydus .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/tmp/nydus.db
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

EXPOSE 15318

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./nydus"]