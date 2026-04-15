DROP INDEX IF EXISTS uq_sessions_user_device_active;
DROP INDEX IF EXISTS idx_users_subscription_end;
DROP INDEX IF EXISTS idx_payments_user_created_at;
DROP INDEX IF EXISTS idx_sessions_user_disconnected;
DROP INDEX IF EXISTS idx_sessions_user_connected_at;
DROP INDEX IF EXISTS idx_configs_user_active;

ALTER TABLE IF EXISTS sessions DROP CONSTRAINT IF EXISTS fk_sessions_server_instance;

DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS configs;
DROP TABLE IF EXISTS server_instances;
DROP TABLE IF EXISTS users;
