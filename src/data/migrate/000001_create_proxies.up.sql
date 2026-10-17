-- 代理表: 数据链路(CRUD)用; 不含 application(与 proxy 解耦)与 endpoint(属 whistle 启动参数, 存 rules)
CREATE TABLE IF NOT EXISTS proxies (
    id          TEXT    PRIMARY KEY,                 -- 应用生成的 ULID(与前端 string id 一致)
    name        TEXT    NOT NULL,                    -- 展示名/唯一标识(未删除时唯一)
    target      TEXT    NOT NULL,                    -- 上游地址 host:port
    status      TEXT    NOT NULL DEFAULT 'stopped',  -- 运行态(数据链路只读写, 不触发执行)
    rules       TEXT    NOT NULL DEFAULT '{}',       -- whistle 启动参数(标准 JSON 字符串)
    description TEXT    NOT NULL DEFAULT '',         -- 描述
    created_at  INTEGER NOT NULL,                    -- unix 毫秒
    updated_at  INTEGER NOT NULL,                    -- unix 毫秒
    deleted_at  INTEGER NOT NULL DEFAULT 0,          -- unix 毫秒; 0 表示未删除(逻辑删除)
    CHECK (status IN ('running', 'stopped', 'error', 'exception'))
);

-- 唯一性只约束未删除的行: 逻辑删除后同名可以重新创建
CREATE UNIQUE INDEX IF NOT EXISTS idx_proxies_name_active ON proxies (name) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_proxies_status ON proxies (status) WHERE deleted_at = 0;
CREATE INDEX IF NOT EXISTS idx_proxies_updated_at ON proxies (updated_at DESC) WHERE deleted_at = 0;
