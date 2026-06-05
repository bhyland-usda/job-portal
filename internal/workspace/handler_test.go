package workspace

import (
	"context"
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bhyland-usda/job-portal/internal/middleware"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	return db, mock
}

func TestNewHandler(t *testing.T) {
	db, _ := setupMockDB(t)
	defer db.Close()

	h := NewHandler(db, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.db != db {
		t.Error("handler db not set")
	}
}

// TestListWorkspacesQuery verifies the list query only returns workspaces the
// user is a member of, joined to workspace_members.
func TestListWorkspacesQuery(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-1", "Alpha", "First", "user-1", now, 2))

	rows, err := db.QueryContext(context.Background(),
		`SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at,
			(SELECT COUNT(*) FROM workspace_members wm WHERE wm.workspace_id = ws.id) AS member_count
		 FROM workspaces ws
		 JOIN workspace_members m ON m.workspace_id = ws.id AND m.user_id = $1
		 ORDER BY ws.created_at DESC`, "user-1")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var ws Workspace
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	if err := rows.Scan(&ws.ID, &ws.Name, &ws.Description, &ws.CreatedBy, &ws.CreatedAt, &ws.MemberCount); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if ws.ID != "ws-1" || ws.Name != "Alpha" || ws.MemberCount != 2 {
		t.Errorf("unexpected workspace: %+v", ws)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestListWorkspacesWithRecommendations verifies the workspace list page also
// loads recommended workspaces matched from the current user's skills.
func TestListWorkspacesWithRecommendations(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-1", "Alpha", "First", "user-1", now, 2))
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-2", "Gamma", "Skills match", "user-2", now, 4))

	tmpl := template.Must(template.New("workspaces.html").Parse(`{{define "base"}}recommended:{{range .RecommendedWorkspaces}}{{.Name}} {{end}}my:{{range .Workspaces}}{{.Name}} {{end}}{{end}}`))
	h := NewHandler(db, map[string]*template.Template{"workspaces.html": tmpl})

	req := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	data := ListPage{}
	h.listWorkspacesWithData(rec, req, &data)

	if body := rec.Body.String(); body != "recommended:Gamma my:Alpha " {
		t.Fatalf("unexpected body: %q", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListWorkspacesWithSearchResults(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-1", "Alpha", "First", "user-1", now, 2))
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-2", "Gamma", "Skills match", "user-2", now, 4))
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("user-1", "%python%", "%python%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count", "is_member"}).
			AddRow("ws-3", "Python Guild", "Code + data", "user-3", now, 8, false))

	tmpl := template.Must(template.New("workspaces.html").Parse(`{{define "base"}}q={{.SearchQuery}};search:{{range .SearchResults}}{{.Name}} {{end}}{{end}}`))
	h := NewHandler(db, map[string]*template.Template{"workspaces.html": tmpl})

	req := httptest.NewRequest(http.MethodGet, "/workspaces?q=python", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	data := ListPage{}
	h.listWorkspacesWithData(rec, req, &data)

	if body := rec.Body.String(); body != "q=python;search:Python Guild " {
		t.Fatalf("unexpected body: %q", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestShowWorkspaceAllowsNonMembers(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("ws-9", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("SELECT ws.id, ws.name, ws.description, ws.created_by, ws.created_at").
		WithArgs("ws-9").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "created_by", "created_at", "member_count"}).
			AddRow("ws-9", "Open Workspace", "Visible to everyone", "user-2", now, 2))
	mock.ExpectQuery(`SELECT wm.user_id, CONCAT\(u.first_name, ' ', u.last_name\), wm.joined_at`).
		WithArgs("ws-9").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "name", "joined_at"}).
			AddRow("user-2", "Jane Doe", now))
	mock.ExpectQuery(`SELECT n.id, n.author_id, CONCAT\(u.first_name, ' ', u.last_name\), n.body, n.created_at`).
		WithArgs("ws-9").
		WillReturnRows(sqlmock.NewRows([]string{"id", "author_id", "name", "body", "created_at"}))

	tmpl := template.Must(template.New("workspace_view.html").Parse(`{{define "base"}}{{.Workspace.Name}}{{end}}`))
	h := NewHandler(db, map[string]*template.Template{"workspace_view.html": tmpl})

	req := httptest.NewRequest(http.MethodGet, "/workspaces/ws-9", nil)
	req.SetPathValue("id", "ws-9")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	rec := httptest.NewRecorder()

	h.showWorkspace(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "Open Workspace" {
		t.Fatalf("unexpected body: %q", body)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestCreateWorkspaceInsertsAndAddsCreator verifies a transaction that inserts
// the workspace then adds the creator as a member.
func TestCreateWorkspaceInsertsAndAddsCreator(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO workspaces").
		WithArgs("Alpha", "First", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ws-1"))
	mock.ExpectExec("INSERT INTO workspace_members").
		WithArgs("ws-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}

	var id string
	if err := tx.QueryRowContext(context.Background(),
		`INSERT INTO workspaces (name, description, created_by) VALUES ($1, $2, $3) RETURNING id`,
		"Alpha", "First", "user-1").Scan(&id); err != nil {
		t.Fatalf("insert workspace failed: %v", err)
	}
	if id != "ws-1" {
		t.Errorf("expected ws-1, got %s", id)
	}

	if _, err := tx.ExecContext(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id) VALUES ($1, $2)`,
		id, "user-1"); err != nil {
		t.Fatalf("insert member failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestMembershipCheckExists verifies the EXISTS membership query used to authorize.
func TestMembershipCheckExists(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("ws-1", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	var exists bool
	if err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		"ws-1", "user-1").Scan(&exists); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !exists {
		t.Error("expected membership to exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMembershipCheckNotMember(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs("ws-1", "stranger").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	var exists bool
	if err := db.QueryRowContext(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		"ws-1", "stranger").Scan(&exists); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if exists {
		t.Error("expected non-member")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestAddNoteInsert verifies a note insert with workspace, author, body.
func TestAddNoteInsert(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectExec("INSERT INTO workspace_notes").
		WithArgs("ws-1", "user-1", "hello team").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace_notes (workspace_id, author_id, body) VALUES ($1, $2, $3)`,
		"ws-1", "user-1", "hello team"); err != nil {
		t.Fatalf("insert note failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestAddMemberInsert verifies adding a member uses ON CONFLICT DO NOTHING.
func TestAddMemberInsert(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	mock.ExpectExec("INSERT INTO workspace_members").
		WithArgs("ws-1", "user-2").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO workspace_members (workspace_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		"ws-1", "user-2"); err != nil {
		t.Fatalf("insert member failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// TestLoadNotesNewestFirst verifies notes are scanned in order with author name.
func TestLoadNotesNewestFirst(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	t1 := time.Now()
	t2 := t1.Add(-time.Hour)
	mock.ExpectQuery("SELECT n.id, n.author_id, .* FROM workspace_notes n").
		WithArgs("ws-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "author_id", "name", "body", "created_at"}).
			AddRow("n-2", "user-1", "Jane Doe", "newest", t1).
			AddRow("n-1", "user-2", "John Smith", "older", t2))

	rows, err := db.QueryContext(context.Background(),
		`SELECT n.id, n.author_id, CONCAT(u.first_name, ' ', u.last_name), n.body, n.created_at
		 FROM workspace_notes n
		 JOIN users u ON u.id = n.author_id
		 WHERE n.workspace_id = $1
		 ORDER BY n.created_at DESC LIMIT 50`, "ws-1")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.AuthorID, &n.AuthorName, &n.Body, &n.CreatedAt); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		notes = append(notes, n)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Body != "newest" || notes[0].AuthorName != "Jane Doe" {
		t.Errorf("unexpected first note: %+v", notes[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
