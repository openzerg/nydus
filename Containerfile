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
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV NYDUS_PORT=15318
ENV NYDUS_DB=/tmp/nydus.db

EXPOSE 15318

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./nydus"]
