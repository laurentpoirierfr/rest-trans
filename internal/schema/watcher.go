package schema

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

type Watcher struct {
	db         *sql.DB
	store      *SchemaStore
	schemas    []string
	interval   time.Duration
	autoNotify bool
}

func NewWatcher(db *sql.DB, store *SchemaStore, schemas []string, interval time.Duration, autoNotify bool) *Watcher {
	return &Watcher{
		db:         db,
		store:      store,
		schemas:    schemas,
		interval:   interval,
		autoNotify: autoNotify,
	}
}

func (w *Watcher) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		slog.Info("schema watcher started", "interval", w.interval)

		for {
			select {
			case <-ctx.Done():
				slog.Info("schema watcher stopped")
				return
			case <-ticker.C:
				w.refresh()
			}
		}
	}()
}

func (w *Watcher) refresh() {
	newSchemas := make(map[string]map[string]*Table)
	newFunctions := make(map[string]map[string]*Function)

	for _, s := range w.schemas {
		s = trimSpace(s)

		tables, err := Introspect(w.db, s)
		if err != nil {
			slog.Warn("hot-reload: failed to introspect tables", "schema", s, "error", err)
			continue
		}
		newSchemas[s] = tables

		funcs, err := IntrospectFunctions(w.db, s)
		if err != nil {
			slog.Warn("hot-reload: failed to introspect functions", "schema", s, "error", err)
			continue
		}
		newFunctions[s] = funcs
	}

	w.detectChanges(newSchemas, newFunctions)

	w.store.ReplaceTables(newSchemas)
	w.store.ReplaceFunctions(newFunctions)
}

func (w *Watcher) detectChanges(newSchemas map[string]map[string]*Table, newFunctions map[string]map[string]*Function) {
	oldTables := w.store.AllTables()
	oldFunctions := w.store.AllFunctions()

	for s, tables := range newSchemas {
		for name, table := range tables {
			old, exists := oldTables[name]
			if !exists {
				slog.Info("hot-reload: new table detected", "schema", s, "table", name)
				if w.autoNotify {
					w.ensureNotifyTrigger(s, name)
				}
				continue
			}
			detectColumnChanges(s, name, old, table)
		}
	}

	for name := range oldTables {
		found := false
		for _, tables := range newSchemas {
			if _, ok := tables[name]; ok {
				found = true
				break
			}
		}
		if !found {
			slog.Info("hot-reload: table removed", "table", name)
		}
	}

	for s, funcs := range newFunctions {
		for name := range funcs {
			if _, exists := oldFunctions[name]; !exists {
				slog.Info("hot-reload: new function detected", "schema", s, "function", name)
			}
		}
	}

	for name := range oldFunctions {
		found := false
		for _, funcs := range newFunctions {
			if _, ok := funcs[name]; ok {
				found = true
				break
			}
		}
		if !found {
			slog.Info("hot-reload: function removed", "function", name)
		}
	}
}

func detectColumnChanges(schema, table string, old, new *Table) {
	for colName, newCol := range new.Columns {
		if _, exists := old.Columns[colName]; !exists {
			slog.Info("hot-reload: new column detected",
				"schema", schema,
				"table", table,
				"column", colName,
				"type", newCol.DataType,
			)
		}
	}

	for colName := range old.Columns {
		if _, exists := new.Columns[colName]; !exists {
			slog.Info("hot-reload: column removed",
				"schema", schema,
				"table", table,
				"column", colName,
			)
		}
	}
}

func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

func (w *Watcher) ensureNotifyTrigger(schema, table string) {
	funcSQL := `CREATE OR REPLACE FUNCTION rest_notify() RETURNS trigger AS $$
DECLARE
    payload jsonb;
    channel text;
BEGIN
    channel := TG_TABLE_SCHEMA || '_' || TG_TABLE_NAME;
    IF TG_OP = 'INSERT' THEN
        payload := jsonb_build_object('schema', TG_TABLE_SCHEMA, 'table', TG_TABLE_NAME, 'op', TG_OP, 'new', to_jsonb(NEW));
    ELSIF TG_OP = 'UPDATE' THEN
        payload := jsonb_build_object('schema', TG_TABLE_SCHEMA, 'table', TG_TABLE_NAME, 'op', TG_OP, 'old', to_jsonb(OLD), 'new', to_jsonb(NEW));
    ELSIF TG_OP = 'DELETE' THEN
        payload := jsonb_build_object('schema', TG_TABLE_SCHEMA, 'table', TG_TABLE_NAME, 'op', TG_OP, 'old', to_jsonb(OLD));
    END IF;
    PERFORM pg_notify('rest_' || channel, payload::text);
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;`

	if _, err := w.db.Exec(funcSQL); err != nil {
		slog.Error("hot-reload: failed to ensure rest_notify() function", "error", err)
		return
	}

	triggerName := table + "_notify"
	triggerSQL := fmt.Sprintf(
		`CREATE TRIGGER %s AFTER INSERT OR UPDATE OR DELETE ON %s.%s FOR EACH ROW EXECUTE FUNCTION rest_notify()`,
		triggerName, schema, table,
	)

	if _, err := w.db.Exec(triggerSQL); err != nil {
		slog.Error("hot-reload: failed to create notify trigger",
			"schema", schema, "table", table, "error", err)
		return
	}

	slog.Info("hot-reload: notify trigger created", "schema", schema, "table", table)
}
