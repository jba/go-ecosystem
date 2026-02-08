-- avoid nulls to simplify interoperation with Go

CREATE TABLE modules (
    id             INTEGER PRIMARY KEY,
    path           TEXT NOT NULL UNIQUE,
    error          TEXT NOT NULL,
    latest_version TEXT NOT NULL,
    info_time      TEXT NOT NULL
);

-- TODO: make modules strict

CREATE TABLE packages (
    module_id INTEGER NOT NULL,
    relative_path TEXT NOT NULL,
    PRIMARY KEY (module_id, relative_path),
    FOREIGN KEY (module_id) REFERENCES modules(id)
);

CREATE TABLE params (
    name  TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;
