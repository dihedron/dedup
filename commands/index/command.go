package index

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dihedron/dedup/commands/base"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/panjf2000/ants/v2"
)

// Index is the command that scans and indexes all cxontents in one or mode directories
// on disk, in order to check if there are duplicate files on disk, and where they are.
type Index struct {
	base.Command
	// Paths is the array of directory paths to scan and index.
	Paths []string `short:"p" long:"path" description:"The directory path(s) to index." required:"true"`
	// Database is the path to the database to open/create on disk.
	Database string `short:"d" long:"database" description:"Path to the database." required:"true" default:"./dedup.db"`
	// Bucket is a label that is given to all entries indexed during this run.
	Bucket string `short:"b" long:"bucket" description:"The bucket to use for indexing the given paths." optional:"true" default:"default"`
}

// Execute is the real implementation of the Version command.
func (cmd *Index) Execute(args []string) error {
	cmd.Init()
	slog.Debug("running index command", "paths", cmd.Paths, "database", cmd.Database)

	// open the SQLite3 database
	db, err := sql.Open("sqlite3", cmd.Database)
	if err != nil {
		slog.Error("error opening SQLite database", "path", cmd.Database, "error", err)
		return err
	}
	defer db.Close()

	// prepare the migrations
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		slog.Error("error loading SQLite migration driver", "error", err)
		return err
	}
	migration, err := migrate.NewWithDatabaseInstance("file://./migrations", "sqlite3", driver)
	if err != nil {
		slog.Error("error creating SQLite migration", "error", err)
		return err
	}
	if err = migration.Up(); err != nil {
		slog.Error("error applying SQLite migration", "error", err)
		return err
	}

	// create the workers' pool
	var wg sync.WaitGroup
	mp, _ := ants.NewMultiPool(10, -1, ants.RoundRobin)
	defer mp.ReleaseTimeout(5 * time.Second)

	// now visit the filesystem
	visit := func(path string, object fs.DirEntry, err error) error {
		if object.Type().IsDir() {
			slog.Debug("visit directory", "path", path)
		} else if object.Type().IsRegular() {
			slog.Debug("visit regular file", "path", path)
			wg.Add(1)
			_ = mp.Submit(func() {
				defer wg.Done()
				f, err := os.Open(path)
				if err != nil {
					slog.Error("error opening file", "path", path, "error", err)
					return
				}
				defer f.Close()

				var size int64
				h := sha256.New()
				if size, err = io.Copy(h, f); err != nil {
					slog.Error("error reading file", "path", path, "error", err)
					return
				}

				hash := hex.EncodeToString(h.Sum(nil))
				slog.Debug("file processed", "path", path, "hash", hash)
				//db.View

				tx, err := db.Begin()
				if err != nil {
					slog.Error("error opening database transaction", "error", err)
					return
				}
				stmt, err := tx.Prepare("insert into entries(hash, path, bucket, size) values(?, ?, ?, ?)")
				if err != nil {
					slog.Error("error preparing database insert statement", "error", err)
					return
				}
				defer stmt.Close()
				_, err = stmt.Exec(hash, path, cmd.Bucket, size)
				if err != nil {
					slog.Error("error executing database insert statement", "error", err)
					return
				}
				if err = tx.Commit(); err != nil {
					slog.Error("error committing database insert transaction", "error", err)
					return
				}
			})
		} else {
			slog.Warn("visit object", "path", path, "type", object.Type().String())
		}
		return nil
	}

	for _, path := range cmd.Paths {
		slog.Debug("visiting directory", "path", path)
		if err := filepath.WalkDir(path, visit); err != nil {
			slog.Error("error visiting directory", "path", path, "error", err)
		}
	}
	slog.Debug("filepath.WalkDir() returned", "error", err)
	// slog.Debug("command done")
	return nil
}
