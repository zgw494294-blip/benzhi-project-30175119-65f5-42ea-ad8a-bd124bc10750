package main

import (
	"context"
	"dialectrelease/internal/repository"
	"dialectrelease/internal/web"
	"dialectrelease/internal/workflow"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	ctx := context.Background()
	dbPath := cfg.dbPath
	if cfg.selfcheck {
		dbPath = ":memory:"
	}
	store, err := repository.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("初始化 SQLite: %w", err)
	}
	defer store.Close()
	svc := workflow.New(store, workflow.RandomIDs{}, time.Now)
	handler := web.New(svc)
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if cfg.selfcheck {
		return executeSelfcheck(server, listener.Addr().String(), serveErr)
	}
	log.Printf("方言语料授权发布工作台已启动：http://%s", listener.Addr())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		log.Printf("收到 %s，开始关闭", sig)
	case err = <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
