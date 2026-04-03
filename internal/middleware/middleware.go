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

// AdminTokenInterceptor validates a static Bearer token for admin access.
// If adminToken is empty, authentication is skipped (development mode).
func AdminTokenInterceptor(adminToken string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if adminToken == "" {
				// No token configured — allow all
				return next(ctx, req)
			}

			authHeader := req.Header().Get("Authorization")
			if authHeader == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing Authorization header"))
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid Authorization header format"))
			}

			if parts[1] != adminToken {
				return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token"))
			}

			return next(ctx, req)
		}
	}
}
