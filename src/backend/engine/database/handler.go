package database

import (
	stdctx "context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
)

// 错误哨兵: 参数类型不符 / 空事务.
var (
	errBadArgs = errors.New("database: invalid task args")
	errNoSteps = errors.New("database: tx requires at least one step")
)

func (e *Engine) handlers() map[domainstore.Cmd]domainstore.Handler {
	return map[domainstore.Cmd]domainstore.Handler{
		domainstore.CmdExec:  e.exec,
		domainstore.CmdQuery: e.query,
		domainstore.CmdGet:   e.get,
		domainstore.CmdTx:    e.tx,
	}
}

func (e *Engine) exec(ctx stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.ExecArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	start := time.Now()
	res, err := e.db.ExecContext(ctx, a.SQL, a.Args...)
	e.logOp("db.exec", a.SQL, a.Args, time.Since(start), err)
	if err != nil {
		return nil, err
	}
	return execResult(res), nil
}

func (e *Engine) query(ctx stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.QueryArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	text, bind, err := e.paginate(a)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if a.Into != nil {
		err := sqlx.SelectContext(ctx, e.db, a.Into, text, bind...)
		e.logOp("db.query", a.SQL, a.Args, time.Since(start), err)
		if err != nil {
			return nil, err
		}
		return domainstore.QueryResult{Count: sliceLen(a.Into)}, nil
	}
	rows, err := e.rows(ctx, text, bind)
	e.logOp("db.query", a.SQL, a.Args, time.Since(start), err)
	if err != nil {
		return nil, err
	}
	return domainstore.QueryResult{Count: len(rows), Rows: rows}, nil
}

func (e *Engine) get(ctx stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.QueryArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	text, bind, err := e.paginate(a)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	rows, err := e.db.QueryxContext(ctx, text, bind...)
	if err != nil {
		e.logOp("db.get", a.SQL, a.Args, time.Since(start), err)
		return nil, err
	}
	defer rows.Close()

	row := domainstore.Row{}
	if rows.Next() {
		if err := rows.MapScan(row); err != nil {
			e.logOp("db.get", a.SQL, a.Args, time.Since(start), err)
			return nil, err
		}
		e.logOp("db.get", a.SQL, a.Args, time.Since(start), nil)
		return row, nil
	}
	if err := rows.Err(); err != nil {
		e.logOp("db.get", a.SQL, a.Args, time.Since(start), err)
		return nil, err
	}
	e.logOp("db.get", a.SQL, a.Args, time.Since(start), sql.ErrNoRows)
	return nil, sql.ErrNoRows
}

func (e *Engine) tx(ctx stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.TxArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	if len(a.Steps) == 0 {
		return nil, errNoSteps
	}
	start := time.Now()
	tx, err := e.db.BeginTxx(ctx, nil)
	if err != nil {
		e.logOp("db.tx", "", nil, time.Since(start), err)
		return nil, err
	}
	out := make([]domainstore.ExecResult, 0, len(a.Steps))
	for _, step := range a.Steps {
		res, err := tx.ExecContext(ctx, step.SQL, step.Args...)
		if err != nil {
			_ = tx.Rollback()
			e.logOp("db.tx", step.SQL, step.Args, time.Since(start), err)
			return nil, err
		}
		out = append(out, execResult(res))
	}
	if err := tx.Commit(); err != nil {
		e.logOp("db.tx", "", nil, time.Since(start), err)
		return nil, err
	}
	e.logOp("db.tx", "", nil, time.Since(start), nil)
	return out, nil
}

// paginate 依据分页游标改写 SQL 并补全绑定参数.
func (e *Engine) paginate(a domainstore.QueryArgs) (string, []any, error) {
	if a.Page == nil {
		return a.SQL, a.Args, nil
	}
	limit := a.Page.Limit
	if limit <= 0 {
		limit = e.maxRows
	}
	if limit > e.maxRows {
		return "", nil, fmt.Errorf("database: page limit %d exceeds max %d", limit, e.maxRows)
	}
	bind := make([]any, 0, len(a.Args)+2)
	bind = append(bind, a.Args...)
	bind = append(bind, limit, a.Page.Offset)
	return strings.TrimRight(a.SQL, "; \t\n") + " LIMIT ? OFFSET ?", bind, nil
}

// rows 返回通用行集 (列名 -> 值).
func (e *Engine) rows(ctx stdctx.Context, text string, bind []any) ([]domainstore.Row, error) {
	rows, err := e.db.QueryxContext(ctx, text, bind...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domainstore.Row, 0, 8)
	for rows.Next() {
		row := domainstore.Row{}
		if err := rows.MapScan(row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func execResult(res sql.Result) domainstore.ExecResult {
	out := domainstore.ExecResult{}
	if n, err := res.RowsAffected(); err == nil {
		out.RowsAffected = n
	}
	if id, err := res.LastInsertId(); err == nil {
		out.LastInsertID = id
	}
	return out
}

// sliceLen 取强类型目标的填充条数.
func sliceLen(v any) int {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0
	}
	if elem := rv.Elem(); elem.Kind() == reflect.Slice {
		return elem.Len()
	}
	return 0
}
