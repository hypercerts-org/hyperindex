DROP INDEX IF EXISTS idx_actor_label_preference_did;
DROP TABLE IF EXISTS actor_label_preference;
-- Note: SQLite doesn't support DROP COLUMN, would need table recreation
