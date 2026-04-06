-- Add translation settings and cached item translations.

CREATE TABLE IF NOT EXISTS ai_translation_settings (
	id                          INTEGER PRIMARY KEY CHECK (id = 1),
	openai_api_key              TEXT NOT NULL DEFAULT '',
	translation_model           TEXT NOT NULL DEFAULT '',
	translation_target_language TEXT NOT NULL DEFAULT ''
);

ALTER TABLE items ADD COLUMN translated_title TEXT;
ALTER TABLE items ADD COLUMN translated_content TEXT;
ALTER TABLE items ADD COLUMN translation_model TEXT;
ALTER TABLE items ADD COLUMN translation_target_language TEXT;
ALTER TABLE items ADD COLUMN translation_updated_at INTEGER NOT NULL DEFAULT 0;
