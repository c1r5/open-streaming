package cmd

import (
	"log"

	"github.com/c1r5/open-streaming/src/shared/config"
	"github.com/c1r5/open-streaming/src/shared/database"
	_ "github.com/c1r5/open-streaming/src/shared/database/sqlite"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(migrateCmd)
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()

		log.Printf("Running migrations on %s\n", cfg.Database.DSN)

		if err := database.Connect(gormlite.Open(cfg.Database.DSN)); err != nil {
			log.Fatalf("migration failed: %v\n", err)
		}

		if err := database.Close(); err != nil {
			log.Fatalf("failed to close database: %v\n", err)
		}

		log.Println("Migrations applied successfully.")
	},
}
