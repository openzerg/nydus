FROM docker.io/library/golang:1.24-alpine

WORKDIR /app

RUN apk add --no-cache gcc musl-dev sqlite-dev bash coreutils ca-certificates git

COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o nydus ./cmd/nydus

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/tmp/nydus.db
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

EXPOSE 15318

CMD ["./nydus"]