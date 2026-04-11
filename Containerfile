FROM docker.io/nixos/nix:latest AS builder

RUN nix --extra-experimental-features "nix-command flakes" --option substituters "https://mirrors.ustc.edu.cn/nix-channels/store https://cache.nixos.org/" --option trusted-public-keys "cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=" profile install github:NixOS/nixpkgs/nixos-25.11#go

ENV PATH=/nix/profiles/default/bin:${PATH}
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o nydus ./cmd/nydus

FROM docker.io/library/debian:trixie-slim

WORKDIR /app

COPY --from=builder /app/nydus .

RUN cat > /entrypoint.sh << 'EOF'
#!/bin/sh
set -e

mkdir -p /root

mkdir -p /etc/ssl/certs
for cert in /nix/store/*-nss-cacert-*/etc/ssl/certs/ca-bundle.crt; do
    if [ -f "$cert" ]; then
        ln -sf "$cert" /etc/ssl/certs/ca-certificates.crt
        export SSL_CERT_FILE="$cert"
        export CURL_CA_BUNDLE="$cert"
        export GIT_SSL_CAINFO="$cert"
        export NIX_SSL_CERT_FILE="$cert"
        break
    fi
done

export NIX_REMOTE=daemon
export NIX_CONFIG="experimental-features = nix-command flakes
flake-registry = /tmp/nix-registry.json"
export XDG_CACHE_HOME=/nix-cache

NIX_BIN=$(ls -d /nix/store/*-nix-2.* 2>/dev/null | grep -v '\.drv$' | grep -v '\.patch$' | head -1)
if [ -n "$NIX_BIN" ]; then
    export PATH="${NIX_BIN}/bin:${PATH}"
fi

mkdir -p "${NYDUS_DB%/*}" 2>/dev/null || true

exec "$@"
EOF
RUN chmod +x nydus /entrypoint.sh

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/var/lib/nydus/nydus.db

EXPOSE 15318

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./nydus"]
