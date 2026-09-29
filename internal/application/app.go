package application

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"resume/internal/database"
	"resume/internal/models"
	pdfservice "resume/internal/pdf"
	"resume/web"
)

type Config struct {
	Addr, DatabasePath, UploadPath, BaseURL, SessionSecret, FontPath string
	SecureCookies                                                    bool
}
type App struct {
	db     *sql.DB
	cfg    Config
	tmpl   *template.Template
	logger *log.Logger
}
type AdminPage struct {
	CSRF, Section, Kind, Error, Notice string
	UserID                             int64
	Languages                          []models.Language
	Selected                           models.Language
	Resume                             models.Resume
	Edit                               *models.Entry
	Dashboard                          models.Dashboard
	WebsiteTemplate, PDFTemplate       string
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func boolEnv(k string) bool {
	v := strings.ToLower(env(k, ""))
	return v == "1" || v == "true" || v == "yes"
}
func config() Config {
	base := strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/")
	return Config{Addr: ":" + env("PORT", "8080"), DatabasePath: env("DATABASE_PATH", "data/resume.db"), UploadPath: env("UPLOAD_PATH", "uploads"), BaseURL: base, SessionSecret: env("SESSION_SECRET", ""), FontPath: env("FONT_PATH", ""), SecureCookies: boolEnv("SECURE_COOKIES") || strings.HasPrefix(base, "https://")}
}

func Run(args []string) error {
	cfg := config()
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	create := fs.Bool("create-admin", false, "create an administrator")
	seed := fs.Bool("seed", false, "insert demo content")
	username := fs.String("username", "", "admin username")
	password := fs.String("password", "", "admin password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if *create {
		return createAdmin(db, *username, *password)
	}
	if *seed {
		return seedDemo(db)
	}
	if cfg.SessionSecret == "" {
		return errors.New("SESSION_SECRET is required (use a long random value)")
	}
	if err = os.MkdirAll(cfg.UploadPath, 0755); err != nil {
		return err
	}
	funcs := template.FuncMap{"section": func(m map[string][]models.Entry, k string) []models.Entry { return m[k] }, "dateRange": dateRange, "checked": func(v bool) string {
		if v {
			return "checked"
		}
		return ""
	}, "selected": func(a, b string) string {
		if a == b {
			return "selected"
		}
		return ""
	}, "dict": func(v ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(v); i += 2 {
			if k, ok := v[i].(string); ok {
				m[k] = v[i+1]
			}
		}
		return m
	}}
	t, err := template.New("root").Funcs(funcs).ParseFS(web.Files, "templates/*.html")
	if err != nil {
		return err
	}
	a := &App{db: db, cfg: cfg, tmpl: t, logger: log.New(os.Stdout, "resume ", log.LstdFlags)}
	database.CleanupSessions(db)
	s := &http.Server{Addr: cfg.Addr, Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	a.logger.Printf("listening on %s", cfg.Addr)
	return s.ListenAndServe()
}

func createAdmin(db *sql.DB, user, pass string) error {
	user = strings.TrimSpace(user)
	if user == "" || len(pass) < 12 {
		return errors.New("use --username and a --password of at least 12 characters")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO users(username,password_hash) VALUES(?,?)", user, string(h))
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	fmt.Println("administrator created")
	return nil
}
func seedDemo(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	langs := []struct {
		c, n, nn, d string
		o           int
	}{{"en", "English", "English", "ltr", 1}, {"de", "German", "Deutsch", "ltr", 2}, {"fa", "Persian", "فارسی", "rtl", 3}}
	for _, l := range langs {
		_, err = tx.Exec("INSERT OR IGNORE INTO languages(code,name,native_name,direction,sort_order) VALUES(?,?,?,?,?)", l.c, l.n, l.nn, l.d, l.o)
		if err != nil {
			return err
		}
	}
	rows, err := tx.Query("SELECT id,code FROM languages")
	if err != nil {
		return err
	}
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var c string
		rows.Scan(&id, &c)
		ids[c] = id
	}
	rows.Close()
	data := map[string][]string{"en": {"Alex Morgan", "Software Engineer", "I build useful, durable software.", "Product-minded engineer focused on Go and simple systems.", "Berlin, Germany"}, "de": {"Alex Morgan", "Softwareentwickler", "Ich entwickle langlebige, nützliche Software.", "Produktorientierter Entwickler mit Fokus auf Go und einfache Systeme.", "Berlin, Deutschland"}, "fa": {"الکس مورگان", "مهندس نرم‌افزار", "نرم‌افزارهای مفید و پایدار می‌سازم.", "مهندس محصول‌محور با تمرکز بر Go و سامانه‌های ساده.", "برلین، آلمان"}}
	for c, v := range data {
		_, err = tx.Exec("INSERT OR REPLACE INTO profile_translations(profile_id,language_id,name,title,intro,about,email,location,meta_description) VALUES(1,?,?,?,?,?,?,?,?)", ids[c], v[0], v[1], v[2], v[3], "hello@example.com", v[4], v[2])
		if err != nil {
			return err
		}
	}
	var count int
	tx.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count)
	if count == 0 {
		res, err := tx.Exec("INSERT INTO entries(kind,start_date,end_date,current,technologies,sort_order) VALUES('experience','2022','',1,'Go, SQLite, TypeScript',10)")
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		titles := map[string][]string{"en": {"Senior Software Engineer", "Acme Studio", "Building reliable products and leading platform work."}, "de": {"Senior Softwareentwickler", "Acme Studio", "Entwicklung zuverlässiger Produkte und Leitung der Plattformarbeit."}, "fa": {"مهندس ارشد نرم‌افزار", "استودیو اکمی", "ساخت محصولات پایدار و هدایت کارهای پلتفرم."}}
		for c, v := range titles {
			tx.Exec("INSERT INTO entry_translations(entry_id,language_id,title,subtitle,description) VALUES(?,?,?,?,?)", id, ids[c], v[0], v[1], v[2])
		}
		for i, s := range []string{"Go", "SQLite", "System Design"} {
			res, _ = tx.Exec("INSERT INTO entries(kind,proficiency,sort_order) VALUES('skill','Advanced',?)", i*10)
			sid, _ := res.LastInsertId()
			for _, lid := range ids {
				tx.Exec("INSERT INTO entry_translations(entry_id,language_id,title) VALUES(?,?,?)", sid, lid, s)
			}
		}
	}
	return tx.Commit()
}

func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	m.Handle("/static/", http.StripPrefix("/", http.FileServer(http.FS(web.Files))))
	m.HandleFunc("/uploads/", a.uploaded)
	m.HandleFunc("/robots.txt", a.robots)
	m.HandleFunc("/sitemap.xml", a.sitemap)
	m.HandleFunc("/admin/login", a.login)
	m.HandleFunc("/admin/logout", a.adminOnly(a.logout))
	m.HandleFunc("/admin", a.adminOnly(a.admin))
	m.HandleFunc("/admin/", a.adminOnly(a.admin))
	m.HandleFunc("/", a.public)
	return a.securityHeaders(m)
}
func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:; form-action 'self'; frame-ancestors 'self'")
		next.ServeHTTP(w, r)
	})
}
func (a *App) uploaded(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	if name == "." || name == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, filepath.Join(a.cfg.UploadPath, name))
}
func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (a *App) tokenHash(v string) string {
	mac := hmac.New(sha256.New, []byte(a.cfg.SessionSecret))
	mac.Write([]byte(v))
	return hex.EncodeToString(mac.Sum(nil))
}
func (a *App) session(r *http.Request) (int64, string, bool) {
	c, err := r.Cookie("resume_session")
	if err != nil {
		return 0, "", false
	}
	var uid int64
	var csrf, exp string
	err = a.db.QueryRow("SELECT user_id,csrf_token,expires_at FROM sessions WHERE id_hash=?", a.tokenHash(c.Value)).Scan(&uid, &csrf, &exp)
	if err != nil {
		return 0, "", false
	}
	t, e := time.Parse(time.RFC3339, exp)
	return uid, csrf, e == nil && t.After(time.Now())
}
func (a *App) setSession(w http.ResponseWriter, uid int64) error {
	raw, csrf := randomToken(), randomToken()
	exp := time.Now().Add(24 * time.Hour).UTC()
	_, err := a.db.Exec("INSERT INTO sessions(id_hash,user_id,csrf_token,expires_at) VALUES(?,?,?,?)", a.tokenHash(raw), uid, csrf, exp.Format(time.RFC3339))
	if err == nil {
		http.SetCookie(w, &http.Cookie{Name: "resume_session", Value: raw, Path: "/", Expires: exp, MaxAge: 86400, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	}
	return err
}
func (a *App) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := a.session(r); !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
func (a *App) validCSRF(r *http.Request) bool {
	_, csrf, ok := a.session(r)
	return ok && csrf != "" && r.FormValue("csrf") == csrf
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.render(w, "login", map[string]string{})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.badRequest(w)
		return
	}
	var id int64
	var hash string
	err := a.db.QueryRow("SELECT id,password_hash FROM users WHERE username=? OR email=?", strings.TrimSpace(r.FormValue("username")), strings.TrimSpace(r.FormValue("username"))).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.FormValue("password"))) != nil {
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(401)
		a.render(w, "login", map[string]string{"Error": "Invalid username or password."})
		return
	}
	if err = a.setSession(w, id); err != nil {
		a.internal(w, err)
		return
	}
	http.Redirect(w, r, "/admin", 303)
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !a.validCSRF(r) {
		a.badRequest(w)
		return
	}
	if c, e := r.Cookie("resume_session"); e == nil {
		a.db.Exec("DELETE FROM sessions WHERE id_hash=?", a.tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "resume_session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/admin/login", 303)
}

func (a *App) public(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		langs, _ := database.EnabledLanguages(a.db)
		if len(langs) == 0 {
			a.friendly(w, 503, "Resume setup is not complete")
			return
		}
		http.Redirect(w, r, "/"+langs[0].Code, 302)
		return
	}
	if strings.HasSuffix(path, "/resume.pdf") {
		a.publicPDF(w, r, strings.TrimSuffix(path, "/resume.pdf"))
		return
	}
	if strings.Contains(path, "/") {
		a.notFound(w)
		return
	}
	lang, err := database.LanguageByCode(a.db, path, false)
	if err != nil {
		a.notFound(w)
		return
	}
	resume, err := database.LoadResume(a.db, lang, a.cfg.BaseURL, false)
	if err != nil {
		a.internal(w, err)
		return
	}
	langs, _ := database.EnabledLanguages(a.db)
	data := struct {
		models.Resume
		Languages []models.Language
		Canonical string
	}{resume, langs, a.cfg.BaseURL + "/" + lang.Code}
	name := "public_" + resume.WebsiteTemplate
	if a.tmpl.Lookup(name) == nil {
		name = "public_modern"
	}
	a.render(w, name, data)
}
func (a *App) publicPDF(w http.ResponseWriter, r *http.Request, code string) {
	lang, err := database.LanguageByCode(a.db, code, false)
	if err != nil {
		a.notFound(w)
		return
	}
	resume, err := database.LoadResume(a.db, lang, a.cfg.BaseURL, false)
	if err != nil {
		a.internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=resume-%s.pdf", lang.Code))
	if err = pdfservice.Write(w, resume, a.cfg.FontPath, a.cfg.UploadPath); err != nil {
		a.logger.Printf("pdf: %v", err)
	}
}

func (a *App) admin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		a.adminPost(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	_, csrf, _ := a.session(r)
	langs, err := database.Languages(a.db, false)
	if err != nil {
		a.internal(w, err)
		return
	}
	code := r.URL.Query().Get("lang")
	if code == "" && len(langs) > 0 {
		code = langs[0].Code
	}
	var selected models.Language
	for _, l := range langs {
		if l.Code == code {
			selected = l
		}
	}
	page := AdminPage{CSRF: csrf, Languages: langs, Selected: selected, Section: strings.TrimPrefix(r.URL.Path, "/admin/")}
	if r.URL.Path == "/admin" || page.Section == "" {
		page.Section = "dashboard"
	}
	page.Kind = r.URL.Query().Get("kind")
	if page.Kind == "" {
		page.Kind = page.Section
	}
	if selected.ID > 0 {
		page.Resume, _ = database.LoadResume(a.db, selected, a.cfg.BaseURL, true)
	}
	page.WebsiteTemplate = database.Setting(a.db, "website_template")
	page.PDFTemplate = database.Setting(a.db, "pdf_template")
	page.Dashboard = a.dashboard()
	if id, _ := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64); id > 0 {
		page.Edit = a.entryForEdit(id, selected.ID)
	}
	a.render(w, "admin", page)
}
func (a *App) dashboard() models.Dashboard {
	var d models.Dashboard
	a.db.QueryRow("SELECT COUNT(*) FROM languages").Scan(&d.Languages)
	for k, p := range map[string]*int{"experience": &d.Experiences, "education": &d.Education, "project": &d.Projects, "skill": &d.Skills} {
		a.db.QueryRow("SELECT COUNT(*) FROM entries WHERE kind=?", k).Scan(p)
	}
	d.WebsiteTemplate = database.Setting(a.db, "website_template")
	d.PDFTemplate = database.Setting(a.db, "pdf_template")
	return d
}
func (a *App) entryForEdit(id, langID int64) *models.Entry {
	var e models.Entry
	err := a.db.QueryRow(`SELECT e.id,e.kind,e.image,e.url,e.secondary_url,e.start_date,e.end_date,e.current,e.proficiency,e.technologies,e.enabled,e.sort_order,COALESCE(t.title,''),COALESCE(t.subtitle,''),COALESCE(t.location,''),COALESCE(t.description,''),COALESCE(t.extra,'') FROM entries e LEFT JOIN entry_translations t ON t.entry_id=e.id AND t.language_id=? WHERE e.id=?`, langID, id).Scan(&e.ID, &e.Kind, &e.Image, &e.URL, &e.SecondaryURL, &e.StartDate, &e.EndDate, &e.Current, &e.Proficiency, &e.Technologies, &e.Enabled, &e.SortOrder, &e.Title, &e.Subtitle, &e.Location, &e.Description, &e.Extra)
	if err != nil {
		return nil
	}
	return &e
}

func (a *App) adminPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		a.badRequest(w)
		return
	}
	if !a.validCSRF(r) {
		a.friendly(w, 403, "Invalid or expired form token")
		return
	}
	action := r.FormValue("action")
	var err error
	switch action {
	case "profile":
		err = a.saveProfile(r)
	case "entry":
		err = a.saveEntry(r)
	case "delete-entry":
		_, err = a.db.Exec("DELETE FROM entries WHERE id=?", r.FormValue("id"))
	case "language":
		err = a.saveLanguage(r)
	case "delete-language":
		_, err = a.db.Exec("DELETE FROM languages WHERE id=?", r.FormValue("id"))
	case "social":
		err = a.saveSocial(r)
	case "delete-social":
		_, err = a.db.Exec("DELETE FROM social_links WHERE id=?", r.FormValue("id"))
	case "settings":
		err = a.saveSettings(r)
	default:
		a.badRequest(w)
		return
	}
	if err != nil {
		a.internal(w, err)
		return
	}
	dest := r.FormValue("return")
	if dest == "" {
		dest = "/admin"
	}
	http.Redirect(w, r, dest, 303)
}
func (a *App) saveProfile(r *http.Request) error {
	lid, err := strconv.ParseInt(r.FormValue("language_id"), 10, 64)
	if err != nil {
		return err
	}
	avatar := ""
	_ = a.db.QueryRow("SELECT avatar FROM profile WHERE id=1").Scan(&avatar)
	if f, h, e := r.FormFile("image"); e == nil {
		defer f.Close()
		avatar, err = a.saveImage(f, h)
		if err != nil {
			return err
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE profile SET avatar=?,updated_at=CURRENT_TIMESTAMP WHERE id=1", avatar); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO profile_translations(profile_id,language_id,name,title,intro,about,email,phone,location,meta_description) VALUES(1,?,?,?,?,?,?,?,?,?) ON CONFLICT(profile_id,language_id) DO UPDATE SET name=excluded.name,title=excluded.title,intro=excluded.intro,about=excluded.about,email=excluded.email,phone=excluded.phone,location=excluded.location,meta_description=excluded.meta_description`, lid, strings.TrimSpace(r.FormValue("name")), strings.TrimSpace(r.FormValue("title")), r.FormValue("intro"), r.FormValue("about"), strings.TrimSpace(r.FormValue("email")), strings.TrimSpace(r.FormValue("phone")), strings.TrimSpace(r.FormValue("location")), r.FormValue("meta_description"))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (a *App) saveEntry(r *http.Request) error {
	lid, _ := strconv.ParseInt(r.FormValue("language_id"), 10, 64)
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	kind := r.FormValue("kind")
	if !validKind(kind) || lid == 0 {
		return errors.New("invalid entry")
	}
	image := r.FormValue("existing_image")
	var err error
	if f, h, e := r.FormFile("image"); e == nil {
		defer f.Close()
		image, err = a.saveImage(f, h)
		if err != nil {
			return err
		}
	}
	enabled, current := r.FormValue("enabled") == "1", r.FormValue("current") == "1"
	order, _ := strconv.Atoi(r.FormValue("sort_order"))
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if id == 0 {
		res, e := tx.Exec(`INSERT INTO entries(kind,image,url,secondary_url,start_date,end_date,current,proficiency,technologies,enabled,sort_order) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, kind, image, r.FormValue("url"), r.FormValue("secondary_url"), r.FormValue("start_date"), r.FormValue("end_date"), current, r.FormValue("proficiency"), r.FormValue("technologies"), enabled, order)
		if e != nil {
			return e
		}
		id, _ = res.LastInsertId()
	} else {
		_, err = tx.Exec(`UPDATE entries SET image=?,url=?,secondary_url=?,start_date=?,end_date=?,current=?,proficiency=?,technologies=?,enabled=?,sort_order=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, image, r.FormValue("url"), r.FormValue("secondary_url"), r.FormValue("start_date"), r.FormValue("end_date"), current, r.FormValue("proficiency"), r.FormValue("technologies"), enabled, order, id)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(`INSERT INTO entry_translations(entry_id,language_id,title,subtitle,location,description,extra) VALUES(?,?,?,?,?,?,?) ON CONFLICT(entry_id,language_id) DO UPDATE SET title=excluded.title,subtitle=excluded.subtitle,location=excluded.location,description=excluded.description,extra=excluded.extra`, id, lid, r.FormValue("title"), r.FormValue("subtitle"), r.FormValue("location"), r.FormValue("description"), r.FormValue("extra"))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func validKind(k string) bool {
	switch k {
	case "experience", "education", "skill", "project", "certification", "language":
		return true
	}
	return false
}
func (a *App) saveLanguage(r *http.Request) error {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	code := strings.ToLower(strings.TrimSpace(r.FormValue("code")))
	dir := r.FormValue("direction")
	if code == "" || (dir != "ltr" && dir != "rtl") {
		return errors.New("invalid language")
	}
	order, _ := strconv.Atoi(r.FormValue("sort_order"))
	enabled := r.FormValue("enabled") == "1"
	if id == 0 {
		_, err := a.db.Exec("INSERT INTO languages(code,name,native_name,direction,enabled,sort_order) VALUES(?,?,?,?,?,?)", code, r.FormValue("name"), r.FormValue("native_name"), dir, enabled, order)
		return err
	}
	_, err := a.db.Exec("UPDATE languages SET code=?,name=?,native_name=?,direction=?,enabled=?,sort_order=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", code, r.FormValue("name"), r.FormValue("native_name"), dir, enabled, order, id)
	return err
}
func (a *App) saveSocial(r *http.Request) error {
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	order, _ := strconv.Atoi(r.FormValue("sort_order"))
	enabled := r.FormValue("enabled") == "1"
	if id == 0 {
		_, err := a.db.Exec("INSERT INTO social_links(label,url,icon,enabled,sort_order) VALUES(?,?,?,?,?)", r.FormValue("label"), r.FormValue("url"), r.FormValue("icon"), enabled, order)
		return err
	}
	_, err := a.db.Exec("UPDATE social_links SET label=?,url=?,icon=?,enabled=?,sort_order=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", r.FormValue("label"), r.FormValue("url"), r.FormValue("icon"), enabled, order, id)
	return err
}
func (a *App) saveSettings(r *http.Request) error {
	site, pdf := r.FormValue("website_template"), r.FormValue("pdf_template")
	if site != "modern" && site != "professional" {
		return errors.New("invalid website template")
	}
	if pdf != "modern" && pdf != "professional" {
		return errors.New("invalid PDF template")
	}
	if err := database.SetSetting(a.db, "website_template", site); err != nil {
		return err
	}
	return database.SetSetting(a.db, "pdf_template", pdf)
}
func (a *App) saveImage(f multipart.File, h *multipart.FileHeader) (string, error) {
	if h.Size > 5<<20 {
		return "", errors.New("image exceeds 5 MB")
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	ct := http.DetectContentType(head[:n])
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif"}[ct]
	if ext == "" {
		return "", errors.New("only JPEG, PNG, and GIF images are allowed")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8000 || cfg.Height > 8000 {
		return "", errors.New("invalid image dimensions")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	name := randomToken()[:24] + ext
	dst, err := os.OpenFile(filepath.Join(a.cfg.UploadPath, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, io.LimitReader(f, 5<<20)); err != nil {
		return "", err
	}
	return "/uploads/" + name, nil
}

func dateRange(e models.Entry) string {
	if e.Current {
		if e.StartDate != "" {
			return e.StartDate + " — Present"
		}
		return "Present"
	}
	if e.StartDate != "" && e.EndDate != "" {
		return e.StartDate + " — " + e.EndDate
	}
	return e.StartDate + e.EndDate
}
func (a *App) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "User-agent: *\nAllow: /\nDisallow: /admin\nSitemap: %s/sitemap.xml\n", a.cfg.BaseURL)
}
func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	langs, _ := database.EnabledLanguages(a.db)
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprint(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">")
	for _, l := range langs {
		fmt.Fprintf(w, "<url><loc>%s/%s</loc></url>", template.HTMLEscapeString(a.cfg.BaseURL), url.PathEscape(l.Code))
	}
	fmt.Fprint(w, "</urlset>")
}
func (a *App) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		a.logger.Printf("template %s: %v", name, err)
	}
}
func (a *App) friendly(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	a.render(w, "error", map[string]any{"Status": status, "Message": msg})
}
func (a *App) notFound(w http.ResponseWriter) {
	a.friendly(w, 404, "The page you requested could not be found.")
}
func (a *App) badRequest(w http.ResponseWriter) {
	a.friendly(w, 400, "The request could not be processed.")
}
func (a *App) internal(w http.ResponseWriter, err error) {
	a.logger.Printf("internal error: %v", err)
	a.friendly(w, 500, "Something went wrong. Please try again.")
}
