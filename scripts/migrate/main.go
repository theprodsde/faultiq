package main

import (
    "context"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "sort"

    "github.com/jackc/pgx/v5"
)

func applied(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
    rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
    if err != nil {
        // assume table missing
        return map[string]bool{}, nil
    }
    defer rows.Close()
    m := map[string]bool{}
    for rows.Next() {
        var v string
        if err := rows.Scan(&v); err != nil {
            return nil, err
        }
        m[v] = true
    }
    return m, nil
}

func main() {
    dsn := os.Getenv("PG_DSN")
    if dsn == "" {
        dsn = "postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable"
    }
    ctx := context.Background()
    conn, err := pgx.Connect(ctx, dsn)
    if err != nil {
        log.Fatalf("pg connect: %v", err)
    }
    defer conn.Close(ctx)

    // ensure schema_migrations table exists
    if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMP WITH TIME ZONE DEFAULT now())`); err != nil {
        log.Fatalf("ensure schema_migrations: %v", err)
    }

    files, err := ioutil.ReadDir("/migrations")
    if err != nil {
        log.Fatalf("read migrations: %v", err)
    }
    names := []string{}
    for _, f := range files {
        if f.IsDir() {
            continue
        }
        names = append(names, f.Name())
    }
    sort.Strings(names)

    appliedMap, err := applied(ctx, conn)
    if err != nil {
        log.Fatalf("applied: %v", err)
    }

    for _, n := range names {
        if appliedMap[n] {
            fmt.Printf("skip %s
", n)
            continue
        }
        p := filepath.Join("/migrations", n)
        b, err := os.ReadFile(p)
        if err != nil {
            log.Fatalf("read %s: %v", p, err)
        }
        fmt.Printf("apply %s\n", n)
        if _, err := conn.Exec(ctx, string(b)); err != nil {
            log.Fatalf("exec %s: %v", n, err)
        }
        if _, err := conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", n); err != nil {
            log.Fatalf("record migration: %v", err)
        }
    }
    fmt.Println("migrations applied")
}
