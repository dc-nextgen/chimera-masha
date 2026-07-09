package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/microsoft/go-mssqldb" // "sqlserver" driver kaydi

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/expression"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/live"
)

// Connector — SQL Server DB connector. db salt-okuma login ile acilmali (savunma katmani 1;
// katman 2 = agent yalniz manifest tool'lari sunar, ham SQL yok). man = CANLI manifest (atomik;
// onboarding apply hot-swap eder) — nil olabilir (yalniz introspect yolu; Call manifest ister).
// db = ATOMIK handle → ekrandan-baglan (Connect) calisirken swap eder (restart yok).
type Connector struct {
	db  atomic.Pointer[sql.DB]
	man *live.Manifest
}

// Open — dsn ile baglar (bos ise BAGLANMADAN baslar; ekrandan Connect bekler). Kimlik YERELDE
// (keychain/env/ekran), asla buluta gitmez. Baglanti dogrulamasi Health/Connect'e birakilir.
func Open(dsn string, man *live.Manifest) (*Connector, error) {
	c := &Connector{man: man}
	if strings.TrimSpace(dsn) != "" {
		db, err := openDB(dsn)
		if err != nil {
			return nil, err
		}
		c.db.Store(db)
	}
	return c, nil
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlserver acilamadi: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

func (c *Connector) handle() *sql.DB { return c.db.Load() }

// Connected — DB kimligi ayarli mi (ucuz; ping yok).
func (c *Connector) Connected() bool { return c.db.Load() != nil }

// Connect — YENI dsn'i ping ile dogrula; basarilıysa eskiyi kapat + swap. Basarisizsa swap YOK (mevcut korunur).
func (c *Connector) Connect(ctx context.Context, dsn string) error {
	db, err := openDB(dsn)
	if err != nil {
		return err
	}
	pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pctx); err != nil {
		db.Close()
		return fmt.Errorf("baglanti dogrulanamadi: %w", err)
	}
	if old := c.db.Swap(db); old != nil {
		old.Close()
	}
	return nil
}

func (c *Connector) Health(ctx context.Context) error {
	db := c.handle()
	if db == nil {
		return fmt.Errorf("DB bagli degil (ekrandan baglan)")
	}
	return db.PingContext(ctx)
}

func (c *Connector) Close() error {
	if db := c.handle(); db != nil {
		return db.Close()
	}
	return nil
}

// Call — tool'u manifest'ten uretilen parametreli SELECT ile calistirir, sonucu sekillendirir,
// per-alan expression'i (maske/format) ON-PREM uygular.
func (c *Connector) Call(ctx context.Context, tool string, args map[string]any) (any, error) {
	if c.man == nil {
		return nil, fmt.Errorf("connector manifest'siz (introspect-only)")
	}
	m := c.man.Get()
	if m == nil {
		return nil, fmt.Errorf("connector manifest henuz yuklenmedi")
	}
	t, ok := m.Tool(tool)
	if !ok {
		return nil, fmt.Errorf("bilinmeyen tool: %q", tool)
	}
	db := c.handle()
	if db == nil {
		return nil, fmt.Errorf("DB bagli degil")
	}
	query, params, out, err := BuildQuery(m, t, args)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("sorgu hatasi: %w", err)
	}
	defer rows.Close()

	if t.Kind == "count" {
		var n int64
		if rows.Next() {
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
		}
		return map[string]any{"count": n}, rows.Err()
	}

	// query: satirlari tara, expression uygula.
	var result []map[string]any
	for rows.Next() {
		dest := make([]any, len(out))
		ptrs := make([]any, len(out))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := make(map[string]any, len(out))
		for i, col := range out {
			rec[col.Field] = expression.Apply(col.Expr, dest[i])
		}
		result = append(result, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"rows": result, "count": len(result)}, nil
}

// Introspect — INFORMATION_SCHEMA'dan sema YAPISI (tablo/kolon/tip). SATIR OKUMAZ.
// Onboarding: bu cikti buluta gider (operator + LLM esleme onerir), veri gitmez (§17.3).
func (c *Connector) Introspect(ctx context.Context) (*connector.Schema, error) {
	const q = `
SELECT c.TABLE_SCHEMA, c.TABLE_NAME, c.COLUMN_NAME, c.DATA_TYPE, c.IS_NULLABLE
FROM INFORMATION_SCHEMA.COLUMNS c
JOIN INFORMATION_SCHEMA.TABLES t
  ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME
WHERE t.TABLE_TYPE = 'BASE TABLE'
ORDER BY c.TABLE_SCHEMA, c.TABLE_NAME, c.ORDINAL_POSITION`
	db := c.handle()
	if db == nil {
		return nil, fmt.Errorf("DB bagli degil (ekrandan baglan)")
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("introspeksiyon hatasi: %w", err)
	}
	defer rows.Close()

	type key struct{ s, n string }
	order := []key{}
	byTable := map[key]*connector.Table{}
	for rows.Next() {
		var sch, tab, col, typ, nullable string
		if err := rows.Scan(&sch, &tab, &col, &typ, &nullable); err != nil {
			return nil, err
		}
		k := key{sch, tab}
		tp, ok := byTable[k]
		if !ok {
			tp = &connector.Table{Schema: sch, Name: tab}
			byTable[k] = tp
			order = append(order, k)
		}
		tp.Columns = append(tp.Columns, connector.Column{Name: col, Type: typ, Nullable: nullable == "YES"})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sc := &connector.Schema{}
	for _, k := range order {
		sc.Tables = append(sc.Tables, *byTable[k])
	}
	return sc, nil
}
