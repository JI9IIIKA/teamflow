package teamflow

import (
	"encoding/json"
	"strings"
	p "teamflow/internal/platform"
	"time"
)

func Register(a *p.App) {
	a.Handle("POST", "/tasks", func(r *p.Request) (any, error) {
		var b struct {
			Title string     `json:"title"`
			DueAt *time.Time `json:"due_at"`
		}
		if e := r.Decode(&b); e != nil {
			return nil, e
		}
		b.Title = strings.TrimSpace(b.Title)
		if b.Title == "" {
			return nil, p.Bad("title required")
		}
		return r.One(`INSERT INTO tasks(tenant,id,title,due_at) VALUES($1,$2,$3,$4) RETURNING *`, r.Tenant, p.ID(), b.Title, b.DueAt)
	})
	a.Handle("GET", "/tasks", func(r *p.Request) (any, error) {
		return r.All(`SELECT * FROM tasks WHERE tenant=$1 AND ($2='' OR status=$2) AND title ILIKE '%'||$3||'%' AND id>$4 ORDER BY id LIMIT 50`, r.Tenant, r.HTTP.URL.Query().Get("status"), r.HTTP.URL.Query().Get("q"), r.HTTP.URL.Query().Get("after"))
	})
	a.Handle("GET", "/tasks/{id}", func(r *p.Request) (any, error) {
		return r.One(`SELECT * FROM tasks WHERE tenant=$1 AND id=$2`, r.Tenant, r.Param("id"))
	})
	a.Handle("PATCH", "/tasks/{id}", func(r *p.Request) (any, error) {
		var b struct {
			Title   string `json:"title"`
			Status  string `json:"status"`
			Version int    `json:"version"`
		}
		if e := r.Decode(&b); e != nil {
			return nil, e
		}
		if b.Version < 1 {
			return nil, p.Bad("version required")
		}
		if b.Status != "open" && b.Status != "doing" && b.Status != "done" {
			return nil, p.Bad("invalid status")
		}
		if strings.TrimSpace(b.Title) == "" {
			return nil, p.Bad("title required")
		}
		id := r.Param("id")
		var version int
		if e := r.Tx.QueryRow(r.Ctx, `SELECT version FROM tasks WHERE tenant=$1 AND id=$2`, r.Tenant, id).Scan(&version); e != nil {
			return nil, e
		}
		if version != b.Version {
			return nil, p.Conflict("stale version")
		}
		if b.Status == "done" {
			var blocked bool
			if e := r.Tx.QueryRow(r.Ctx, `SELECT EXISTS(SELECT 1 FROM task_dependencies d JOIN tasks t ON t.tenant=d.tenant AND t.id=d.depends_on WHERE d.tenant=$1 AND d.task_id=$2 AND t.status<>'done')`, r.Tenant, id).Scan(&blocked); e != nil {
				return nil, e
			}
			if blocked {
				return nil, p.Conflict("unfinished dependencies")
			}
		} else {
			var used bool
			if e := r.Tx.QueryRow(r.Ctx, `SELECT EXISTS(SELECT 1 FROM task_dependencies d JOIN tasks t ON t.tenant=d.tenant AND t.id=d.task_id WHERE d.tenant=$1 AND d.depends_on=$2 AND t.status='done')`, r.Tenant, id).Scan(&used); e != nil {
				return nil, e
			}
			if used {
				return nil, p.Conflict("completed task depends on this task")
			}
		}
		result, e := r.One(`UPDATE tasks SET title=$3,status=$4,version=version+1 WHERE tenant=$1 AND id=$2 RETURNING *`, r.Tenant, id, b.Title, b.Status)
		if e != nil {
			return nil, e
		}
		change, _ := json.Marshal(b)
		if e = r.Exec(`INSERT INTO task_history(tenant,task_id,version,actor,change) VALUES($1,$2,$3,$4,$5)`, r.Tenant, id, version+1, r.User, string(change)); e != nil {
			return nil, e
		}
		return result, r.Event("task.updated", result)
	})
	a.Handle("POST", "/tasks/{id}/dependencies", func(r *p.Request) (any, error) {
		var b struct {
			DependsOn string `json:"depends_on"`
		}
		if e := r.Decode(&b); e != nil {
			return nil, e
		}
		id := r.Param("id")
		if id == b.DependsOn {
			return nil, p.Conflict("self dependency")
		}
		var cyclic bool
		if e := r.Tx.QueryRow(r.Ctx, `WITH RECURSIVE chain(id) AS (SELECT depends_on FROM task_dependencies WHERE tenant=$1 AND task_id=$2 UNION SELECT d.depends_on FROM task_dependencies d JOIN chain c ON d.task_id=c.id WHERE d.tenant=$1) SELECT EXISTS(SELECT 1 FROM chain WHERE id=$3)`, r.Tenant, b.DependsOn, id).Scan(&cyclic); e != nil {
			return nil, e
		}
		if cyclic {
			return nil, p.Conflict("dependency cycle")
		}
		var status string
		if e := r.Tx.QueryRow(r.Ctx, `SELECT status FROM tasks WHERE tenant=$1 AND id=$2`, r.Tenant, id).Scan(&status); e != nil {
			return nil, e
		}
		if status == "done" {
			return nil, p.Conflict("reopen task before editing dependencies")
		}
		return nil, r.Exec(`INSERT INTO task_dependencies(tenant,task_id,depends_on) VALUES($1,$2,$3)`, r.Tenant, id, b.DependsOn)
	})
	a.Handle("GET", "/tasks/{id}/history", func(r *p.Request) (any, error) {
		return r.All(`SELECT * FROM task_history WHERE tenant=$1 AND task_id=$2 ORDER BY version`, r.Tenant, r.Param("id"))
	})
	a.Handle("DELETE", "/tasks/{id}", func(r *p.Request) (any, error) {
		return r.One(`DELETE FROM tasks WHERE tenant=$1 AND id=$2 RETURNING id`, r.Tenant, r.Param("id"))
	})
}
