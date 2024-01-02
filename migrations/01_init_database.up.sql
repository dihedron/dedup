CREATE TABLE entries (
    hash    TEXT NOT NULL,
    path    TEXT NOT NULL,
    bucket  TEXT,
    size    INT,
    PRIMARY KEY(hash, path)
);

CREATE INDEX idx_entries_hash 
ON entries (hash);