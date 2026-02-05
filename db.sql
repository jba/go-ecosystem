CREATE TABLE modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    state TEXT NOT NULL
) STRICT;

CREATE TABLE packages (
    module_id INTEGER NOT NULL,
    relative_path TEXT NOT NULL,
    FOREIGN KEY (module_id) REFERENCES modules(id)
) STRICT;

CREATE TABLE params (
    name TEXT NOT NULL,
    value TEXT NOT NULL
) STRICT;
