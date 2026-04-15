ALTER TABLE configs
    ADD COLUMN IF NOT EXISTS tunnel_mode TEXT NOT NULL DEFAULT 'full_tunnel'
    CHECK (tunnel_mode IN ('full_tunnel', 'split_tunnel', 'per_app'));
