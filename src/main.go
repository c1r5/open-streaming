package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/c1r5/open-streaming/src/api"
	"github.com/c1r5/open-streaming/src/config"
	"github.com/c1r5/open-streaming/src/core/database"
	_ "github.com/c1r5/open-streaming/src/core/database/sqlite"
	"github.com/c1r5/open-streaming/src/core/engine"
	"github.com/c1r5/open-streaming/src/modules/streaming"
	"github.com/c1r5/open-streaming/src/modules/torrent"
	"github.com/c1r5/open-streaming/src/pkgs/search"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
)

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.Get()

	if dir := filepath.Dir(cfg.Database.DSN); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal("Failed to create database directory:", err)
		}
	}

	if err := database.Connect(gormlite.Open(cfg.Database.DSN)); err != nil {
		log.Fatalf("cannot connect database: %v\n", err)
	}

	done := make(chan bool, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()

	ia := api.CreateIMDB(cfg.IMDB.BaseURL)
	ta := api.CreateTorrentio(cfg.Torrentio.BaseURL, cfg.Torrentio.UserAgent)

	ts := search.CreateTorrentSearch(ta, ia)

	eng, err := engine.CreateTorrentEngine(ia, ta, engine.EngineOptions{
		DataDir:         cfg.TorrentEngine.DataDir,
		ReadaheadMB:     cfg.TorrentEngine.ReadaheadMB,
		CacheTTLMinutes: cfg.TorrentEngine.CacheTTLMinutes,
	})
	if err != nil {
		log.Fatalf("cannot create torrent engine: %v\n", err)
	}

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	db := database.GetInstance()
	torrent.New(mux, ts, eng, db)
	streaming.New(mux, eng, db)

	handler := withRecovery(withLogging(mux))

	s := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        handler,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:   0,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	go func() {
		sig := <-sigs
		log.Println("\nReceived signal:", sig)
		done <- true
	}()

	go func() {
		log.Printf("Server listening on :%s\n", cfg.Server.Port)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error:", err)
		}
	}()

	<-done
	log.Println("Program terminated gracefully.")
}
