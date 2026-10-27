-- Analysis: 操作与日志追踪(供 Home > Analysis 计算指标; 见 docs/design/v25.md §3)
CREATE TABLE IF NOT EXISTS proxy_trace (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    proxy_id TEXT    NOT NULL,
    name     TEXT    NOT NULL,
    listen   TEXT    NOT NULL DEFAULT '',
    action   TEXT    NOT NULL,
    created_at       INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_proxy_trace_at ON proxy_trace(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_proxy_trace_action ON proxy_trace(action, created_at DESC);

CREATE TABLE IF NOT EXISTS log_trace (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    level   TEXT    NOT NULL,
    domain  TEXT    NOT NULL,
    message TEXT    NOT NULL,
    extra   TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_log_trace_at ON log_trace(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_trace_level ON log_trace(level, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_trace_domain ON log_trace(domain, created_at DESC);

CREATE TABLE IF NOT EXISTS bridge_trace (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    service TEXT    NOT NULL,
    ok      INTEGER NOT NULL,
    created_at      INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bridge_trace_at ON bridge_trace(service, created_at DESC);
