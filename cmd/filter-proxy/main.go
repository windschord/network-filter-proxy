// Network Filter Proxy
//
//	@title						Network Filter Proxy Management API
//	@version					1.0.0
//	@description				Management REST API for the Network Filter Proxy. Provides CRUD operations for whitelist rule sets and a health check endpoint.
//	@host						127.0.0.1:8080
//	@BasePath					/
//	@license.name				MIT
//	@securityDefinitions.basic	none
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/claudework/network-filter-proxy/internal/api"
	"github.com/claudework/network-filter-proxy/internal/config"
	"github.com/claudework/network-filter-proxy/internal/logger"
	"github.com/claudework/network-filter-proxy/internal/proxy"
	"github.com/claudework/network-filter-proxy/internal/rule"
)

var version = "dev"

func run() int {
	cfg := config.Load()
	log := logger.New(cfg.LogFormat, cfg.LogLevel)

	store := rule.NewStore()
	proxyHandler := proxy.NewHandler(store, log)
	apiHandler := api.NewHandler(store, log, proxyHandler)

	proxySrv := &http.Server{
		Addr:    ":" + cfg.ProxyPort,
		Handler: proxyHandler,
	}
	apiSrv := &http.Server{
		Addr:    "127.0.0.1:" + cfg.APIPort,
		Handler: apiHandler.Routes(),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		log.Info("proxy server starting", "port", cfg.ProxyPort, "version", version)
		if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("proxy server error", "err", err)
			errCh <- err
		}
	}()
	go func() {
		log.Info("api server starting", "addr", "127.0.0.1:"+cfg.APIPort)
		if err := apiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("api server error", "err", err)
			errCh <- err
		}
	}()

	exitCode := 0
	select {
	case <-ctx.Done():
	case err := <-errCh:
		log.Error("server failed, initiating shutdown", "err", err)
		stop()
		exitCode = 1
	}

	log.Info("shutdown initiated")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, srv := range []*http.Server{proxySrv, apiSrv} {
		wg.Add(1)
		go func(s *http.Server) {
			defer wg.Done()
			if err := s.Shutdown(shutdownCtx); err != nil {
				log.Error("shutdown error", "err", err, slog.String("addr", s.Addr))
			}
		}(srv)
	}
	wg.Wait()
	proxyHandler.Shutdown()
	log.Info("shutdown complete")
	return exitCode
}

func main() {
	os.Exit(run())
}
