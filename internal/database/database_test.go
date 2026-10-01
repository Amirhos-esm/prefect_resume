package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLanguageCalendarAndResumeDisplayDates(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "resume.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := db.Exec("INSERT INTO languages(code,name,native_name,direction,calendar) VALUES('fa','Persian','فارسی','rtl','shamsi')")
	if err != nil {
		t.Fatal(err)
	}
	languageID, _ := result.LastInsertId()
	result, err = db.Exec("INSERT INTO entries(kind,start_date,end_date) VALUES('experience','2024-03-20','2025-03-20')")
	if err != nil {
		t.Fatal(err)
	}
	entryID, _ := result.LastInsertId()
	if _, err = db.Exec("INSERT INTO entry_translations(entry_id,language_id,title) VALUES(?,?,?)", entryID, languageID, "تجربه"); err != nil {
		t.Fatal(err)
	}

	language, err := LanguageByCode(db, "fa", false)
	if err != nil {
		t.Fatal(err)
	}
	if language.Calendar != "shamsi" {
		t.Fatalf("calendar = %q, want shamsi", language.Calendar)
	}
	resume, err := LoadResume(db, language, "http://example.test", false)
	if err != nil {
		t.Fatal(err)
	}
	entry := resume.Entries["experience"][0]
	if entry.StartDate != "2024-03-20" || entry.DisplayStartDate != "1403/01/01" {
		t.Fatalf("stored/display start dates = %q/%q", entry.StartDate, entry.DisplayStartDate)
	}
}

func TestOpenMigratesLegacyLanguagesTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE languages(
		id INTEGER PRIMARY KEY,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		native_name TEXT NOT NULL,
		direction TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	); INSERT INTO languages(code,name,native_name,direction) VALUES('en','English','English','ltr');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	language, err := LanguageByCode(db, "en", false)
	if err != nil {
		t.Fatal(err)
	}
	if language.Calendar != "gregorian" {
		t.Fatalf("migrated calendar = %q, want gregorian", language.Calendar)
	}
}
