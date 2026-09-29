package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"resume/internal/models"
)

const Schema = `
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE, email TEXT UNIQUE COLLATE NOCASE, password_hash TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS sessions(id_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, csrf_token TEXT NOT NULL, expires_at TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS languages(id INTEGER PRIMARY KEY, code TEXT NOT NULL UNIQUE COLLATE NOCASE, name TEXT NOT NULL, native_name TEXT NOT NULL, direction TEXT NOT NULL CHECK(direction IN ('ltr','rtl')), enabled INTEGER NOT NULL DEFAULT 1, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS profile(id INTEGER PRIMARY KEY CHECK(id=1), avatar TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS profile_translations(profile_id INTEGER NOT NULL REFERENCES profile(id) ON DELETE CASCADE, language_id INTEGER NOT NULL REFERENCES languages(id) ON DELETE CASCADE, name TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', intro TEXT NOT NULL DEFAULT '', about TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '', phone TEXT NOT NULL DEFAULT '', location TEXT NOT NULL DEFAULT '', meta_description TEXT NOT NULL DEFAULT '', PRIMARY KEY(profile_id,language_id));
CREATE TABLE IF NOT EXISTS entries(id INTEGER PRIMARY KEY, kind TEXT NOT NULL CHECK(kind IN ('experience','education','skill','project','certification','language')), image TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '', secondary_url TEXT NOT NULL DEFAULT '', start_date TEXT NOT NULL DEFAULT '', end_date TEXT NOT NULL DEFAULT '', current INTEGER NOT NULL DEFAULT 0, proficiency TEXT NOT NULL DEFAULT '', technologies TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 1, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS entry_translations(entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE, language_id INTEGER NOT NULL REFERENCES languages(id) ON DELETE CASCADE, title TEXT NOT NULL DEFAULT '', subtitle TEXT NOT NULL DEFAULT '', location TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', extra TEXT NOT NULL DEFAULT '', PRIMARY KEY(entry_id,language_id));
CREATE TABLE IF NOT EXISTS social_links(id INTEGER PRIMARY KEY, label TEXT NOT NULL, url TEXT NOT NULL, icon TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 1, sort_order INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_entries_kind_order ON entries(kind,enabled,sort_order);
CREATE INDEX IF NOT EXISTS idx_entry_translations_language ON entry_translations(language_id);
INSERT OR IGNORE INTO profile(id) VALUES(1);
INSERT OR IGNORE INTO settings(key,value) VALUES('website_template','modern');
INSERT OR IGNORE INTO settings(key,value) VALUES('pdf_template','professional');
INSERT OR IGNORE INTO schema_migrations(version,applied_at) VALUES(1,CURRENT_TIMESTAMP);
`

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(Schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func Setting(db *sql.DB, key string) string {
	var v string
	_ = db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v
}
func SetSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	return err
}
func EnabledLanguages(db *sql.DB) ([]models.Language, error) { return Languages(db, true) }
func Languages(db *sql.DB, enabledOnly bool) ([]models.Language, error) {
	q := "SELECT id,code,name,native_name,direction,enabled,sort_order FROM languages"
	if enabledOnly {
		q += " WHERE enabled=1"
	}
	q += " ORDER BY sort_order,name"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Language
	for rows.Next() {
		var x models.Language
		if err = rows.Scan(&x.ID, &x.Code, &x.Name, &x.NativeName, &x.Direction, &x.Enabled, &x.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func LanguageByCode(db *sql.DB, code string, includeDisabled bool) (models.Language, error) {
	var x models.Language
	q := "SELECT id,code,name,native_name,direction,enabled,sort_order FROM languages WHERE code=?"
	if !includeDisabled {
		q += " AND enabled=1"
	}
	err := db.QueryRow(q, code).Scan(&x.ID, &x.Code, &x.Name, &x.NativeName, &x.Direction, &x.Enabled, &x.SortOrder)
	return x, err
}
func LoadResume(db *sql.DB, lang models.Language, baseURL string, includeDisabled bool) (models.Resume, error) {
	r := models.Resume{Language: lang, Entries: map[string][]models.Entry{}, WebsiteTemplate: Setting(db, "website_template"), PDFTemplate: Setting(db, "pdf_template"), BaseURL: baseURL}
	err := db.QueryRow(`SELECT p.avatar,COALESCE(t.name,''),COALESCE(t.title,''),COALESCE(t.intro,''),COALESCE(t.about,''),COALESCE(t.email,''),COALESCE(t.phone,''),COALESCE(t.location,''),COALESCE(t.meta_description,'') FROM profile p LEFT JOIN profile_translations t ON t.profile_id=p.id AND t.language_id=? WHERE p.id=1`, lang.ID).Scan(&r.Profile.Avatar, &r.Profile.Name, &r.Profile.Title, &r.Profile.Intro, &r.Profile.About, &r.Profile.Email, &r.Profile.Phone, &r.Profile.Location, &r.Profile.MetaDescription)
	if err != nil {
		return r, err
	}
	q := `SELECT e.id,e.kind,e.image,e.url,e.secondary_url,e.start_date,e.end_date,e.current,e.proficiency,e.technologies,e.enabled,e.sort_order,COALESCE(t.title,''),COALESCE(t.subtitle,''),COALESCE(t.location,''),COALESCE(t.description,''),COALESCE(t.extra,'') FROM entries e LEFT JOIN entry_translations t ON t.entry_id=e.id AND t.language_id=?`
	if !includeDisabled {
		q += " WHERE e.enabled=1"
	}
	q += " ORDER BY e.kind,e.sort_order,e.id"
	rows, err := db.Query(q, lang.ID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var e models.Entry
		if err = rows.Scan(&e.ID, &e.Kind, &e.Image, &e.URL, &e.SecondaryURL, &e.StartDate, &e.EndDate, &e.Current, &e.Proficiency, &e.Technologies, &e.Enabled, &e.SortOrder, &e.Title, &e.Subtitle, &e.Location, &e.Description, &e.Extra); err != nil {
			return r, err
		}
		r.Entries[e.Kind] = append(r.Entries[e.Kind], e)
	}
	sq := "SELECT id,label,url,icon,enabled,sort_order FROM social_links"
	if !includeDisabled {
		sq += " WHERE enabled=1"
	}
	sq += " ORDER BY sort_order,id"
	srows, err := db.Query(sq)
	if err != nil {
		return r, err
	}
	defer srows.Close()
	for srows.Next() {
		var s models.Social
		if err = srows.Scan(&s.ID, &s.Label, &s.URL, &s.Icon, &s.Enabled, &s.SortOrder); err != nil {
			return r, err
		}
		r.Socials = append(r.Socials, s)
	}
	return r, nil
}
func CleanupSessions(db *sql.DB) {
	db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UTC().Format(time.RFC3339))
}
