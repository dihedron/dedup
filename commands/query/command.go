package query

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dihedron/dedup/commands/base"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Query is the command that runs queries against the current database.
type Query struct {
	base.Command
	// Database is the path to the database to open/create on disk.
	Database string `short:"d" long:"database" description:"Path to the database." required:"true" default:"./dedup.db"`
}

// Execute is the real implementation of the Version command.
func (cmd *Query) Execute(queries []string) error {
	cmd.Init()
	slog.Debug("running query command", "queries", queries)

	if len(queries) == 0 {
		slog.Error("no queries provided")
		return errors.New("no queries provided")
	}

	// open the SQLite3 database
	db, err := sql.Open("sqlite3", cmd.Database+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		slog.Error("error opening SQLite database", "path", cmd.Database, "error", err)
		return err
	}
	defer db.Close()

	for _, query := range queries {
		rows, err := db.Query(query)
		if err != nil {
			slog.Error("error running query", "query", query, "error", err)
			return err
		}

		t := table.NewWriter()
		t.SetTitle("QUERY: " + query)
		t.Style().Format.Header = text.FormatTitle
		t.SetAutoIndex(true)

		// get column names
		columns, err := rows.Columns()
		if err != nil {
			slog.Error("error retrieving columns from query metadata", "error", err)
			return err
		}
		headers := table.Row{}
		for _, column := range columns {
			headers = append(headers, column)
		}
		t.AppendHeader(headers)

		// make a slice for the values
		values := make([]sql.RawBytes, len(columns))

		// rows.Scan wants '[]any' as an argument, so we must copy the
		// references into such a slice
		// See http://code.google.com/p/go-wiki/wiki/InterfaceSlice for details
		scanArgs := make([]any, len(values))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		// fetch rows
		for rows.Next() {
			// get RawBytes from data
			err = rows.Scan(scanArgs...)
			if err != nil {
				slog.Error("error retrieving row values from row set", "error", err)
				return err
			}

			scanValues := make([]any, len(values))
			for i, col := range values {
				if col == nil {
					scanValues[i] = "NULL"
				} else {
					scanValues[i] = string(col)
				}
			}
			t.AppendRow(scanValues)
		}
		if err = rows.Err(); err != nil {
			slog.Error("error reading rows from database", "error", err)
			return err
		}

		fmt.Println(t.Render())

	}

	return nil
}
