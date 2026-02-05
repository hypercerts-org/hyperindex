DROP INDEX IF EXISTS idx_jetstream_activity_rkey;
ALTER TABLE jetstream_activity DROP COLUMN rkey;
