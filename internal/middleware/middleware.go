package middleware

import (
	"context"
	"fmt"
	"net"
	"strings"

	"connectrpc.com/connect"
)

type contextKey string

const (
	InstanceIDKey contextKey = "instance_id"
	SourceIPKey   contextKey = "source_ip"
)

func getRemoteIP(req connect.AnyRequest) string {
	forwarded := req.Header().Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	peer := req.Peer()
	if peer.Addr != "" {
		host, _, err := net.SplitHostPort(peer.Addr)
		if err == nil {
			return host
		}
		return peer.Addr
	}
	return ""
}

func ExtractSourceIPInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			remoteIP := getRemoteIP(req)
			ctx = context.WithValue(ctx, SourceIPKey, remoteIP)
			return next(ctx, req)
		}
	}
}

func CerebrateOnlyInterceptor(cerebrateIP string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			remoteIP := getRemoteIP(req)
			if remoteIP != cerebrateIP {
				return nil, connect.NewError(connect.CodePermissionDenied,
					fmt.Errorf("access denied: only Cerebrate can call this method"))
			}
			return next(ctx, req)
		}
	}
}
