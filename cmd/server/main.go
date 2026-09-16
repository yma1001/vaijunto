package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/server"
	"github.com/yma1001/vaijunto/internal/store"
)

func main() {
	cfg := config.Load()
	st, err := store.New(cfg.DataPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	srv := server.New(cfg, st, nil)

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		log.Printf("shutdown")
		_ = srv.Close()
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
