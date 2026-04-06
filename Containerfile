FROM nixos/nix:latest

WORKDIR /app

RUN nix-env -iA nixpkgs.go nixpkgs.gcc nixpkgs.sqlite nixpkgs.bash nixpkgs.coreutils nixpkgs.cacert

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o nydus ./cmd/nydus

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/tmp/nydus.db
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt

EXPOSE 15318

CMD ["./nydus"]