CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    subscription_end TIMESTAMPTZ,
    tariff TEXT NOT NULL DEFAULT 'basic' CHECK (tariff IN ('basic', 'standard', 'pro')),
    max_devices INTEGER NOT NULL DEFAULT 1 CHECK (max_devices > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS configs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wg_private_key TEXT NOT NULL,
    vk_link TEXT,
    current_creds_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disconnected_at TIMESTAMPTZ,
    ip INET,
    traffic_bytes BIGINT NOT NULL DEFAULT 0 CHECK (traffic_bytes >= 0),
    server_instance_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount >= 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'paid', 'failed', 'refunded', 'cancelled')),
    provider TEXT NOT NULL CHECK (provider IN ('telegram_stars', 'yoomoney', 'robokassa', 'usdt_trc20')),
    provider_payment_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS server_instances (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    region TEXT,
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE sessions
    ADD CONSTRAINT fk_sessions_server_instance
    FOREIGN KEY (server_instance_id)
    REFERENCES server_instances(id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_configs_user_active ON configs(user_id, active);
CREATE INDEX IF NOT EXISTS idx_sessions_user_connected_at ON sessions(user_id, connected_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_disconnected ON sessions(user_id, disconnected_at);
CREATE INDEX IF NOT EXISTS idx_payments_user_created_at ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_users_subscription_end ON users(subscription_end);

CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_user_device_active
    ON sessions(user_id, device_id)
    WHERE disconnected_at IS NULL;
