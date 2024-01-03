package index

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/dihedron/dedup/commands/base"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
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
	// Parallelism represents the number of parallel goroutines to use for digesting files.
	Parallelism int `long:"parallelism" description:"The number of parallel goroutines to use for digesting files." optional:"true" default:"20"`
}

// Execute is the real implementation of the Version command.
func (cmd *Index) Execute(args []string) error {
	cmd.Init()
	slog.Debug("running index command", "paths", cmd.Paths, "database", cmd.Database)

	// open the SQLite3 database
	db, err := sql.Open("sqlite3", cmd.Database+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		slog.Error("error opening SQLite database", "path", cmd.Database, "error", err)
		return err
	}
	defer db.Close()

	stmt := `
	CREATE TABLE IF NOT EXISTS entries (
		hash    TEXT NOT NULL,
		path    TEXT NOT NULL,
		bucket  TEXT,
		size    INT,
		PRIMARY KEY(hash, path)
	);
	CREATE INDEX IF NOT EXISTS idx_entries_hash ON entries (hash);
	`
	_, err = db.Exec(stmt)
	if err != nil {
		slog.Error("error creating table", "error", err)
		return err
	}

	for _, path := range cmd.Paths {
		err := func(path string) error {
			// the entries channel provides all the entries as they're processed
			entries := make(chan entry)
			// the done channel is closed when the path visit ends; it may do so
			// before receiving all the values from c and errc.
			done := make(chan struct{})
			defer close(done)

			// visit the directories starting at path
			slog.Debug("starting directory tree visit...", "path", path)
			paths, errs := visit(done, path)

			// start a fixed number of goroutines to read and digest files
			var wg sync.WaitGroup
			wg.Add(cmd.Parallelism)
			slog.Debug("starting file digesters...", "parallelism", cmd.Parallelism)
			for i := 0; i < cmd.Parallelism; i++ {
				i := i
				go func() {
					slog.Debug("starting digester...", "index", i)
					digest(cmd.Bucket, done, paths, entries)
					slog.Debug("digester done!", "index", i)
					wg.Done()
				}()
			}
			go func() {
				slog.Debug("waiting for all digesters to complete...")
				wg.Wait()
				slog.Debug("all digesters done!")
				close(entries)
			}()

			// now loop over the entries as they flow in
			for e := range entries {
				if e.err != nil {
					slog.Error("error processing entry", "path", e.Path, "error", e.err)
					continue
				} else {
					slog.Info("storing entry into database...", "entry", e.String())
					err := func(e entry) error {
						tx, err := db.Begin()
						if err != nil {
							// slog.Error("error opening database transaction", "error", err)
							return err
						}
						stmt, err := tx.Prepare("INSERT OR REPLACE INTO entries(hash, path, bucket, size) values(?, ?, ?, ?)")
						if err != nil {
							// slog.Error("error preparing database insert statement", "error", err)
							return err
						}
						defer stmt.Close()
						_, err = stmt.Exec(e.Hash, e.Path, e.Bucket, e.Size)
						if err != nil {
							// slog.Error("error executing database insert statement", "error", err)
							return err
						}
						if err = tx.Commit(); err != nil {
							// slog.Error("error committing database insert transaction", "error", err)
							return err
						}
						return nil
					}(e)
					if err != nil {
						slog.Error("error storing entry into database...", "entry", e.String(), "error", err)
					} else {
						slog.Info("entry stored into database...", "entry", e.String())
					}
				}
			}
			// check whether the walk failed.
			if err := <-errs; err != nil {
				slog.Error("error walking directory tree", "path", path, "error,", err)
				return err
			}
			return nil
		}(path)
		if err != nil {
			slog.Error("directory tree visit failed", "path", path, "error", err)
			return err
		}
	}

	// slog.Debug("command done")
	return nil
}

// visit starts a goroutine to walk the directory tree at root and send the
// path of each regular file on the string channel; it sends the result of the
// walk on the error channel; if done is closed, visit abandons its work.
func visit(done <-chan struct{}, root string) (<-chan string, <-chan error) {
	paths := make(chan string)
	errs := make(chan error, 1)
	slog.Info("starting directory tree visit in separate goroutine...", "path", root)
	go func() {
		// close the paths channel after Walk returns
		defer close(paths)
		// no select needed for this send, since errs is buffered.
		errs <- filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				slog.Debug("filesystem object is not a regular file", "path", path)
				return nil
			}
			select {
			case paths <- path:
				slog.Debug("sending path down the pipeline for further processing...", "path", path)
			case <-done:
				slog.Warn("filesystem visit cancelled!", "path", path)
				return errors.New("walk canceled")
			}
			return nil
		})
	}()
	slog.Info("directory tree visit started in separate goroutine", "path", root)
	return paths, errs
}

// A result is the product of reading and summing a file using MD5.
type entry struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Bucket string `json:"bucket"`
	Size   int64  `json:"size"`
	err    error
}

func (e *entry) String() string {
	d, err := json.Marshal(e)
	if err != nil {
		return ""
	}
	return string(d)
}

// digest reads path names from paths and sends digests of the corresponding
// files on c until either paths or done is closed.
func digest(bucket string, done <-chan struct{}, paths <-chan string, c chan<- entry) {
	for path := range paths {
		hash, size, err := func(path string) (string, int64, error) {
			f, err := os.Open(path)
			if err != nil {
				slog.Error("error opening file", "path", path, "error", err)
				return "", 0, err
			}

			defer f.Close()

			var size int64
			h := sha256.New()
			if size, err = io.Copy(h, f); err != nil {
				slog.Error("error reading file", "path", path, "error", err)
				return "", 0, err
			}

			return hex.EncodeToString(h.Sum(nil)), size, nil
		}(path)
		select {
		case c <- entry{path, hash, bucket, size, err}:
		case <-done:
			return
		}
	}
}
