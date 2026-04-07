ALTER TABLE ai_translation_settings ADD COLUMN auto_translate_mode INTEGER NOT NULL DEFAULT 0;
ALTER TABLE items ADD COLUMN translated_excerpt TEXT;
