package query

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/dihedron/dedup/commands/base"
)

// Query is the command that runs queries against the current database.
type Query struct {
	base.Command
	// Database is the path to the database to open/create on disk.
	Database string `short:"d" long:"database" description:"Path to the database." required:"true" default:"./dedup.db"`
}

// Execute is the real implementation of the Version command.
func (cmd *Query) Execute(args []string) error {
	cmd.Init()
	slog.Debug("running query command", "args", args)

	// open the SQLite3 database
	db, err := sql.Open("sqlite3", cmd.Database+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		slog.Error("error opening SQLite database", "path", cmd.Database, "error", err)
		return err
	}
	defer db.Close()

	for _, arg := range args {
		rows, err := db.Query(arg)
		if err != nil {
			slog.Error("error running query", "query", arg, "error", err)
			return err
		}

		fmt.Println("-----------------------------------")

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			panic(err.Error()) // proper error handling instead of panic in your app
		}

		// Make a slice for the values
		values := make([]sql.RawBytes, len(columns))

		// rows.Scan wants '[]interface{}' as an argument, so we must copy the
		// references into such a slice
		// See http://code.google.com/p/go-wiki/wiki/InterfaceSlice for details
		scanArgs := make([]interface{}, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		// Fetch rows
		for rows.Next() {
			// get RawBytes from data
			err = rows.Scan(scanArgs...)
			if err != nil {
				panic(err.Error()) // proper error handling instead of panic in your app
			}

			// Now do something with the data.
			// Here we just print each column as a string.
			var value string
			for i, col := range values {
				// Here we can check if the value is nil (NULL value)
				if col == nil {
					value = "NULL"
				} else {
					value = string(col)
				}
				fmt.Println(columns[i], ": ", value)
			}
			fmt.Println("-----------------------------------")
		}
		if err = rows.Err(); err != nil {
			panic(err.Error()) // proper error handling instead of panic in your app
		}

	}

	return nil
}
