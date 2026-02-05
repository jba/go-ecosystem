CREATE TABLE modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL
) STRICT;

CREATE TABLE packages (
    module_id INTEGER NOT NULL,
    relative_path TEXT NOT NULL,
    PRIMARY KEY (module_id, relative_path),
    FOREIGN KEY (module_id) REFERENCES modules(id)
);

CREATE TABLE params (
    name TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;
