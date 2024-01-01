package index

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
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
	Database string `short:"d" long:"directory" description:"Path to the database." required:"true" default:"./dedup.db"`
}

// Execute is the real implementation of the Version command.
func (cmd *Index) Execute(args []string) error {
	cmd.Init()
	slog.Debug("running index command", "paths", cmd.Paths, "database", cmd.Database)

	// open the data file; it will be created if it doesn't exist
	// db, err := bolt.Open(cmd.Database, 0600, &bolt.Options{Timeout: 1 * time.Second})
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()

	db, err := sql.Open("sqlite3", cmd.Database)
	if err != nil {
		slog.Error("error opening SQLite database", "path", cmd.Database, "error", err)
		return err
	}
	defer db.Close()

	// sqlStmt := `
	// create table entry (hash text not null, path text, size int);
	// `
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		slog.Error("error loading SQLite migration driver", "error", err)
		return err
	}
	migration, err := migrate.NewWithDatabaseInstance("file://./migrations", "sqlite3", driver)
	if err != nil {
		slog.Error("error creatting SQLite migration", "error", err)
		return err
	}
	migration.Up()

	// create the workers' pool
	var wg sync.WaitGroup
	mp, _ := ants.NewMultiPool(10, -1, ants.RoundRobin)
	defer mp.ReleaseTimeout(5 * time.Second)
	// for i := 0; i < runTimes; i++ {
	// 	wg.Add(1)
	// 	_ = mp.Submit(syncCalculateSum)
	// }
	// wg.Wait()
	// fmt.Printf("running goroutines: %d\n", mp.Running())
	// fmt.Printf("finish all tasks.\n")

	visit := func(path string, object fs.DirEntry, err error) error {
		if object.Type().IsDir() {
			slog.Debug("visit directory", "path", path)
		} else if object.Type().IsRegular() {
			slog.Debug("visit regular file", "path", path)
			wg.Add(1)
			_ = mp.Submit(func() {
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
					log.Fatal(err)
				}
				stmt, err := tx.Prepare("insert into entries(hash, path, size) values(?, ?, ?)")
				if err != nil {
					log.Fatal(err)
				}
				defer stmt.Close()
				_, err = stmt.Exec(path, hash, size)
				if err != nil {
					log.Fatal(err)
				}
				err = tx.Commit()

				wg.Done()
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
