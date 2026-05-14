CREATE TABLE IF NOT EXISTS logs (
    id BIGSERIAL PRIMARY KEY,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL,
    nodes_count INTEGER NOT NULL DEFAULT 0,
    ports_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
    id BIGSERIAL PRIMARY KEY,
    log_id BIGINT NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (log_id, external_id)
);

CREATE TABLE IF NOT EXISTS ports (
    id BIGSERIAL PRIMARY KEY,
    node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    mac TEXT,
    ip TEXT,
    status TEXT NOT NULL,
    speed TEXT
);

CREATE TABLE IF NOT EXISTS nodes_info (
    id BIGSERIAL PRIMARY KEY,
    node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_nodes_log_id ON nodes(log_id);
CREATE INDEX IF NOT EXISTS idx_ports_node_id ON ports(node_id);
CREATE INDEX IF NOT EXISTS idx_nodes_info_node_id ON nodes_info(node_id);

