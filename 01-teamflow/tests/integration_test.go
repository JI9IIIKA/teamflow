package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"sync"
	p "teamflow/internal/platform"
	"teamflow/migrations"
	"teamflow/projects/teamflow"
	"testing"
)

var sharedDB *pgxpool.Pool
var sharedErr error
var dbOnce sync.Once

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedDB != nil {
		sharedDB.Close()
	}
	os.Exit(code)
}

type fixture struct {
	t      *testing.T
	app    *p.App
	db     *pgxpool.Pool
	tenant string
}

func setup(t *testing.T, name string, register func(*p.App)) *fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set: integration test requires PostgreSQL")
	}
	dbOnce.Do(func() { sharedDB, sharedErr = pgxpool.New(context.Background(), url) })
	if sharedErr != nil {
		t.Fatal(sharedErr)
	}
	db := sharedDB
	var e error
	if e = migrations.Apply(context.Background(), db, name); e != nil {
		t.Fatal(e)
	}
	tenant := p.ID()
	tables := map[string][]string{
		"teamflow": {"task_dependencies", "task_history", "tasks"}, "bookslot": {"bookings", "resources"},
		"jobrunner": {"jobs"}, "notifyhub": {"delivery_attempts", "notifications"},
		"ordercore": {"payment_events", "order_items", "orders", "products"}, "eventscope": {"analytics_events"},
		"flagcontrol": {"flag_heads", "flags"}, "uptimewatch": {"checks", "incidents", "monitors"},
		"knowledgedesk": {"document_access", "chunks", "documents"}, "supportflow": {"ticket_audit", "support_replies", "tickets"},
	}
	t.Cleanup(func() {
		for _, table := range append(tables[name], "outbox", "idempotency", "event_receipts") {
			if _, err := db.Exec(context.Background(), "DELETE FROM "+table+" WHERE tenant=$1", tenant); err != nil {
				t.Errorf("cleanup %s: %v", table, err)
			}
		}
	})

	a := p.New(name, db, map[string]p.Principal{"admin-token-123456": {Tenant: tenant, User: "alice", Role: "admin"}, "editor-token-12345": {Tenant: tenant, User: "bob", Role: "editor"}, "viewer-token-12345": {Tenant: tenant, User: "reader", Role: "viewer"}, "other-token-123456": {Tenant: p.ID(), User: "mallory", Role: "admin"}})
	register(a)
	return &fixture{t, a, db, tenant}
}
func (f *fixture) call(method, path, token, key string, body any) (int, map[string]any) {
	f.t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	f.app.Router.ServeHTTP(w, req)
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result
}
func (f *fixture) ok(method, path string, body any) map[string]any {
	f.t.Helper()
	code, out := f.call(method, path, "admin-token-123456", "", body)
	if code != 200 {
		f.t.Fatalf("%s %s: %d %v", method, path, code, out)
	}
	return out
}
func (f *fixture) sql(q string, args ...any) {
	f.t.Helper()
	if _, e := f.db.Exec(context.Background(), q, args...); e != nil {
		f.t.Fatal(e)
	}
}
func (f *fixture) count(q string, args ...any) int {
	f.t.Helper()
	var n int
	if e := f.db.QueryRow(context.Background(), q, args...).Scan(&n); e != nil {
		f.t.Fatal(e)
	}
	return n
}
func require(t *testing.T, ok bool, msg string) {
	t.Helper()
	if !ok {
		t.Fatal(msg)
	}
}
func TestTeamFlow(t *testing.T) {
	f := setup(t, "teamflow", teamflow.Register)
	x := f.ok("POST", "/tasks", map[string]any{"title": "A"})
	y := f.ok("POST", "/tasks", map[string]any{"title": "B"})
	id := x["id"].(string)
	other := y["id"].(string)
	f.ok("POST", "/tasks/"+id+"/dependencies", map[string]any{"depends_on": other})
	code, _ := f.call("POST", "/tasks/"+other+"/dependencies", "admin-token-123456", "", map[string]any{"depends_on": id})
	require(t, code == 409, "cycle accepted")
	code, _ = f.call("PATCH", "/tasks/"+id, "admin-token-123456", "", map[string]any{"title": "A", "status": "done", "version": 1})
	require(t, code == 409, "blocked task completed")
	f.ok("PATCH", "/tasks/"+other, map[string]any{"title": "B", "status": "done", "version": 1})
	f.ok("PATCH", "/tasks/"+id, map[string]any{"title": "A", "status": "done", "version": 1})
	code, _ = f.call("PATCH", "/tasks/"+id, "admin-token-123456", "", map[string]any{"title": "A", "status": "done", "version": 1})
	require(t, code == 409, "stale edit accepted")
	code, _ = f.call("GET", "/tasks/"+id, "other-token-123456", "", nil)
	require(t, code == 404, "tenant leak")
	code, _ = f.call("POST", "/tasks", "viewer-token-12345", "", map[string]string{"title": "forbidden"})
	require(t, code == 403, "viewer wrote task")
}

func TestIdempotencyAndAuth(t *testing.T) {
	f := setup(t, "teamflow", teamflow.Register)
	first := map[string]string{"title": "one"}
	code, a := f.call("POST", "/tasks", "admin-token-123456", "same", first)
	require(t, code == 200, "create failed")
	code, b := f.call("POST", "/tasks", "admin-token-123456", "same", first)
	require(t, code == 200 && a["id"] == b["id"], "request was not deduplicated")
	code, _ = f.call("POST", "/tasks", "admin-token-123456", "same", map[string]string{"title": "two"})
	require(t, code == 409, "different payload reused key")
	code, _ = f.call("GET", "/tasks", "wrong", "", nil)
	require(t, code == 401, "invalid token accepted")
}
