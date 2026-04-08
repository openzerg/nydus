package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/openzerg/common/client"
	cerebratev1 "github.com/openzerg/common/gen/cerebrate/v1"
	nydusv1connect "github.com/openzerg/common/gen/nydus/v1/nydusv1connect"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/openzerg/nydus/internal/config"
	"github.com/openzerg/nydus/internal/handler"
	"github.com/openzerg/nydus/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	h := handler.New(st)

	mux := http.NewServeMux()
	path, hdlr := nydusv1connect.NewNydusServiceHandler(h, connect.WithInterceptors())
	mux.Handle(path, hdlr)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: c.Handler(h2c.NewHandler(mux, &http2.Server{})),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[nydus] listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	if cfg.CerebrateURL != "" && cfg.AdminToken != "" {
		go registerWithCerebrate(ctx, cfg)
	}

	<-ctx.Done()
	log.Println("[nydus] shutting down")
	_ = srv.Shutdown(context.Background())
}

func registerWithCerebrate(ctx context.Context, cfg *config.Config) {
	cc := client.NewCerebrateClient(cfg.CerebrateURL)

	resp, err := cc.Login(ctx, cfg.AdminToken)
	if err != nil {
		log.Printf("[nydus] cerebrate login failed: %v", err)
		return
	}
	cc.SetToken(resp.UserToken)

	ip := localIP()
	publicURL := cfg.PublicURL
	if publicURL == "" {
		publicURL = fmt.Sprintf("http://%s:%d", ip, cfg.Port)
	}

	info, err := cc.RegisterInstance(ctx, &cerebratev1.RegisterInstanceRequest{
		Name:         "nydus",
		InstanceType: "nydus",
		Ip:           ip,
		Port:         int32(cfg.Port),
		Status:       "running",
		Labels:       map[string]string{"public_url": publicURL},
	})
	if err != nil {
		log.Printf("[nydus] cerebrate register failed: %v", err)
		return
	}
	log.Printf("[nydus] registered with cerebrate, instance_id=%s", info.InstanceId)

	// Send heartbeat every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := cc.Heartbeat(ctx, info.InstanceId); err != nil {
				log.Printf("[nydus] heartbeat failed: %v", err)
				if r, e := cc.Login(ctx, cfg.AdminToken); e == nil {
					cc.SetToken(r.UserToken)
				}
			}
		}
	}
}

func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
