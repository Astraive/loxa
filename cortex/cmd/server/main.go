package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	loxacortex "github.com/astraive/loxa/cortex"
	"github.com/astraive/loxa/cortex/internal/collectorsync"
	"github.com/astraive/loxa/cortex/internal/config"
	"github.com/astraive/loxa/cortex/internal/logging"
	"github.com/astraive/loxa/cortex/internal/processor"
	"github.com/astraive/loxa/cortex/internal/storage"
	grpcserver "github.com/astraive/loxa/cortex/server/grpc"
	httpserver "github.com/astraive/loxa/cortex/server/http"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var version = loxacortex.Version

var grpcServer *grpc.Server

var (
	configPath = flag.String("config", "", "Path to configuration file")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	logFormat  = flag.String("log-format", "json", "Log format (json, console)")
)

func main() {
	flag.Parse()

	logger := logging.New(*logLevel, *logFormat)
	log.Logger = logger.GetZerologLogger()

	log.Info().Msg("Starting LOXA Cortex")

	var (
		cfg *config.Config
		err error
	)
	if *configPath == "" {
		cfg, err = config.LoadDefault()
	} else {
		cfg, err = config.Load(*configPath)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	if *configPath != "" {
		log.Info().Str("config", *configPath).Msg("Loaded configuration")
	}

	stor, err := storage.NewStorage(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer stor.Close()

	log.Info().Msg("Storage initialized")

	if cfg.Collector.SourceOfTruth && cfg.Collector.Mode == "pull" {
		syncProcessor := processor.NewEventProcessor(stor.Events(), stor.Topology(), stor.Graph())
		go collectorsync.RunSourceOfTruthSync(context.Background(), cfg.Collector, syncProcessor)
		log.Info().
			Str("collector_url", cfg.Collector.URL).
			Bool("tail_enabled", cfg.Collector.TailEnabled).
			Msg("Collector source-of-truth sync started")
	}

	if cfg.GRPC.Enabled {
		go startGRPCServer(cfg, stor)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("Starting HTTP server")

	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      httpserver.Handler(cfg, stor),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	<-sigChan
	log.Info().Msg("Shutting down server...")

	if grpcServer != nil {
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
			log.Info().Msg("gRPC server stopped")
		case <-time.After(10 * time.Second):
			log.Warn().Msg("gRPC graceful stop timed out, forcing")
			grpcServer.Stop()
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown error")
	}
	log.Info().Msg("Server shutdown complete")
}

func startGRPCServer(cfg *config.Config, stor storage.Storage) {
	grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	log.Info().Str("addr", grpcAddr).Msg("Starting gRPC server")

	var opts []grpc.ServerOption
	if cfg.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			log.Error().Err(err).Msg("Failed to load TLS certs")
			return
		}
		opts = append(opts, grpc.Creds(creds))
	}

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create listener")
		return
	}

	server := grpc.NewServer(opts...)
	grpcServer = server
	grpcSvc := grpcserver.New(cfg, stor)
	grpcSvc.RegisterServer(server)

	log.Info().Str("addr", grpcAddr).Msg("gRPC server ready")
	if err := server.Serve(listener); err != nil {
		log.Error().Err(err).Msg("gRPC server failed")
	}
}
