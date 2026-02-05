-- Add rkey column to jetstream_activity for linking to records
ALTER TABLE jetstream_activity ADD COLUMN rkey TEXT;

-- Add index for efficient lookups by rkey
CREATE INDEX IF NOT EXISTS idx_jetstream_activity_rkey ON jetstream_activity(rkey);
