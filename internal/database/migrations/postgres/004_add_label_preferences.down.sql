DROP INDEX IF EXISTS idx_actor_label_preference_did;
DROP TABLE IF EXISTS actor_label_preference;
ALTER TABLE label_definition DROP COLUMN default_visibility;
