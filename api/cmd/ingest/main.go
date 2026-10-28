// Command ingest loads registry CSV snapshots into Postgres.
//
//	go run ./cmd/ingest -agencies ../data/real/agencies.csv -flagged ../data/real/flagged.csv
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/edkariuki/kazi-halali/api/internal/store"
)

func main() {
	agencies := flag.String("agencies", "", "path to agencies.csv")
	flagged := flag.String("flagged", "", "path to flagged.csv")
	flag.Parse()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx := context.Background()
	st, err := store.NewPostgres(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	if *agencies != "" {
		rows, err := store.ReadAgenciesCSV(*agencies)
		if err != nil {
			log.Fatal(err)
		}
		n, err := st.UpsertAgencies(ctx, rows)
		if err != nil {
			log.Fatalf("agencies: inserted %d before error: %v", n, err)
		}
		log.Printf("agencies: upserted %d rows from %s", n, *agencies)
	}
	if *flagged != "" {
		rows, err := store.ReadFlaggedCSV(*flagged)
		if err != nil {
			log.Fatal(err)
		}
		n, err := st.UpsertFlagged(ctx, rows)
		if err != nil {
			log.Fatalf("flagged: inserted %d before error: %v", n, err)
		}
		log.Printf("flagged: upserted %d rows from %s", n, *flagged)
	}
}
