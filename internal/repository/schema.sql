CREATE TABLE IF NOT EXISTS projects (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    context     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS labels (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT '',
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS issues (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id          INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    body                TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done', 'cancelled')),
    priority            TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    assignee_session_id TEXT,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    completed_at        TEXT
);

CREATE INDEX IF NOT EXISTS idx_issues_project_status ON issues(project_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_issues_assignee ON issues(assignee_session_id);

CREATE TABLE IF NOT EXISTS issue_labels (
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label_id INTEGER NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id   INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author     TEXT NOT NULL DEFAULT '',
    session_id TEXT,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_comments_issue ON comments(issue_id, created_at);

CREATE TABLE IF NOT EXISTS issue_relations (
    issue_id         INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    related_issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    relation_type    TEXT NOT NULL CHECK (relation_type IN ('relates_to', 'blocks')),
    created_at       TEXT NOT NULL,
    PRIMARY KEY (issue_id, related_issue_id, relation_type)
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS idx_relations_related ON issue_relations(related_issue_id);
