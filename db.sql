CREATE TABLE modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    state TEXT NOT NULL
);

CREATE TABLE packages (
    module_id INTEGER NOT NULL,
    relative_path TEXT NOT NULL,
    FOREIGN KEY (module_id) REFERENCES modules(id)
);
