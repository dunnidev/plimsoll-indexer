// Command devdb runs a throwaway Postgres for local development, so the
// indexer can be run without installing Postgres or Docker.
//
//	go run ./cmd/devdb
//	DATABASE_URL=postgres://plimsoll:plimsoll@localhost:54329/plimsoll?sslmode=disable
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

const port = 54329

func main() {
	dataDir := filepath.Join(".devdb", "data")
	db := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("plimsoll").
		Password("plimsoll").
		Database("plimsoll").
		Port(port).
		DataPath(dataDir).
		RuntimePath(filepath.Join(".devdb", "runtime")).
		BinariesPath(filepath.Join(".devdb", "bin")))
	if err := db.Start(); err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	fmt.Printf("postgres ready: postgres://plimsoll:plimsoll@localhost:%d/plimsoll?sslmode=disable\n", port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	if err := db.Stop(); err != nil {
		log.Fatalf("stop postgres: %v", err)
	}
}
