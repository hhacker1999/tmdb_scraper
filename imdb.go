package main

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

const (
	ImdbURL        = "https://datasets.imdbws.com/title.ratings.tsv.gz"
	UpdateInterval = 12 * time.Hour
)

type IMDBImporter struct {
	DB      *sql.DB
	DataDir string
}

func NewIMDbImporter(db *sql.DB, dataDir string) *IMDBImporter {
	return &IMDBImporter{
		DB:      db,
		DataDir: dataDir,
	}
}

func (i *IMDBImporter) Start(ctx context.Context) error {
	if err := os.MkdirAll(i.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data dir: %v", err)
		return err
	}

	log.Println("Starting initial sync...")
	if err := i.runSync(ctx); err != nil {
		log.Printf("Initial sync failed: %v", err)
		return err
	}

	ticker := time.NewTicker(UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Starting scheduled sync...")
			if err := i.runSync(ctx); err != nil {
				log.Printf("Scheduled sync failed: %v", err)
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// runSync orchestrates the download and database update
func (i *IMDBImporter) runSync(ctx context.Context) error {
	gzPath := filepath.Join(i.DataDir, "title.ratings.tsv.gz")

	// 1. Download
	if err := i.downloadFile(ctx, gzPath); err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	defer os.Remove(gzPath) // Cleanup zip file after processing

	// 2. Initialize Schema
	if err := i.initSchema(); err != nil {
		return fmt.Errorf("schema init error: %w", err)
	}

	// 3. Process and Insert
	if err := i.processAndInsert(ctx, gzPath); err != nil {
		return fmt.Errorf("processing error: %w", err)
	}

	log.Println("Sync completed successfully.")
	return nil
}

func (i *IMDBImporter) downloadFile(ctx context.Context, destPath string) error {
	log.Println("Downloading IMDb ratings...")
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := http.NewRequest(http.MethodGet, ImdbURL, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func (i *IMDBImporter) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS imdb_ratings (
		tconst VARCHAR(15) PRIMARY KEY,
		average_rating FLOAT,
		num_votes INTEGER
	);`
	_, err := i.DB.Exec(query)
	return err
}

func (i *IMDBImporter) processAndInsert(ctx context.Context, gzPath string) error {
	log.Println("Processing file and streaming to DB...")

	// Open file
	f, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Open Gzip reader
	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	// Parse TSV
	reader := csv.NewReader(gr)
	reader.Comma = '\t'
	reader.LazyQuotes = true // Handle messy quotes if any

	// Skip Header
	if _, err := reader.Read(); err != nil {
		return err
	}

	// Begin Transaction
	txn, err := i.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Defer rollback (noop if committed)
	defer txn.Rollback()

	// 1. Create Temp Table (Drop on Commit ensures it cleans up)
	_, err = txn.Exec(`
		CREATE TEMP TABLE temp_ratings (
			tconst VARCHAR(15),
			average_rating FLOAT,
			num_votes INTEGER
		) ON COMMIT DROP;
	`)
	if err != nil {
		return err
	}

	// 2. Prepare COPY statement (This is the fastest way in Go 'lib/pq')
	stmt, err := txn.PrepareContext(
		ctx,
		pq.CopyIn("temp_ratings", "tconst", "average_rating", "num_votes"),
	)
	if err != nil {
		return err
	}

	// 3. Stream rows
	rowCount := 0
outer:
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			record, err := reader.Read()
			if err == io.EOF {
				break outer
			}
			if err != nil {
				return err
			}

			// record[0] = tconst, [1] = avgRating, [2] = numVotes
			if len(record) < 3 {
				continue outer
			}

			// Convert types
			rating, _ := strconv.ParseFloat(record[1], 64)
			votes, _ := strconv.Atoi(record[2])

			// Feed to COPY statement
			_, err = stmt.Exec(record[0], rating, votes)
			if err != nil {
				return err
			}
			rowCount++
		}
	}

	// Flush the COPY buffer
	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return err
	}
	err = stmt.Close()
	if err != nil {
		return err
	}

	log.Printf("Streamed %d rows to temp table. Performing upsert...", rowCount)

	// 4. UPSERT from Temp to Main
	upsertQuery := `
		INSERT INTO imdb_ratings (tconst, average_rating, num_votes)
		SELECT tconst, average_rating, num_votes FROM temp_ratings
		ON CONFLICT (tconst) DO UPDATE 
		SET average_rating = EXCLUDED.average_rating,
			num_votes = EXCLUDED.num_votes;
	`
	_, err = txn.ExecContext(ctx, upsertQuery)
	if err != nil {
		return err
	}

	return txn.Commit()
}
