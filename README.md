# Resume Studio

Resume Studio is a single-process Go resume/CV application with a public multilingual website, protected admin editor, SQLite storage, local image uploads, and on-demand PDF generation. Resume records are independent of both website and PDF designs.

## Requirements

- Go 1.22+
- A Unicode TrueType font for non-Latin PDFs. Arial/Tahoma on Windows and DejaVu Sans on common Linux distributions are detected automatically.

## Fresh installation

PowerShell:

```powershell
Copy-Item .env.example .env
$env:SESSION_SECRET = "replace-with-a-long-random-secret"
go mod download
go run ./cmd/server --create-admin --username admin --password "a-long-unique-password"
go run ./cmd/server --seed
go run ./cmd/server
```

The `--seed` command is optional. It adds English, German, and Persian demo translations and a few example records. It is safe to omit. Open `http://localhost:8080/admin/login` to edit the resume.

Environment variables are not automatically loaded from `.env`; export them in your shell or use Docker Compose. This avoids adding a configuration dependency.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_PATH` | `data/resume.db` | SQLite file |
| `UPLOAD_PATH` | `uploads` | Uploaded images |
| `BASE_URL` | `http://localhost:8080` | Canonical URLs and sitemap |
| `SESSION_SECRET` | required | Long random application secret |
| `SECURE_COOKIES` | inferred from HTTPS | Force Secure cookies |
| `FONT_PATH` | auto-detected | Unicode TTF used in PDFs |

## Routes

- `/` redirects to the first enabled language.
- `/{language}` renders the active website template.
- `/{language}/resume.pdf` generates the current PDF on demand.
- `/admin` provides the dashboard and editors.
- `/robots.txt` and `/sitemap.xml` provide basic search-engine metadata.

Languages are data, not code: add a code, display names, text direction, and translations in the admin panel. Shared fields such as dates, images, URLs, ordering, and visibility remain attached to one core record while each language stores only translated text.

## Data and security

- bcrypt password hashes; no built-in password
- random opaque sessions stored as SHA-256 hashes
- HttpOnly, SameSite=Strict cookies and optional Secure flag
- CSRF tokens on every authenticated mutation
- parameterized SQL and escaped HTML templates
- 5 MB upload limit, MIME and image-dimension validation, generated filenames
- foreign keys, indexes, WAL mode, ordering and visibility controls

Back up both `data/resume.db` and `uploads/`. SQLite is appropriate for one application instance; do not run several containers against the same database file.

## Docker

Create `.env` containing at least `SESSION_SECRET`, then:

```bash
docker compose build
docker compose run --rm resume --create-admin --username admin --password 'a-long-unique-password'
docker compose run --rm resume --seed
docker compose up -d
```

The database and uploads use named persistent volumes. The image includes DejaVu Sans for Unicode PDF output.

## Development checks

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/server
```

Schema versioning is recorded in `schema_migrations`; startup applies the idempotent migration in `internal/database`. The review copy is under `migrations/`.
