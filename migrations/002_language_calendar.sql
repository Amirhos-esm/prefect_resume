ALTER TABLE languages
ADD COLUMN calendar TEXT NOT NULL DEFAULT 'gregorian'
CHECK(calendar IN ('gregorian','shamsi'));

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES(3, CURRENT_TIMESTAMP);
