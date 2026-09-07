# Deployment Issues & Solutions

**Date:** 2026-09-07  
**Environment:** Production (Alibaba Cloud ECS + Neon PostgreSQL)  
**Status:** ✅ RESOLVED

---

## Issues Encountered During Deployment

| # | Issue | Root Cause | Status | Fix Applied |
|---|-------|------------|--------|-------------|
| 1 | Go backend crash loop | IPv6 DNS resolution - server can't reach Neon via IPv6 | ✅ Fixed | Added `extra_hosts` in docker-compose to force IPv4 |
| 2 | Wrong DB password | Duplicate `POSTGRES_PASSWORD` in .env - wrong one (neondb_owner's) overwrote correct one (app_user's) | ✅ Fixed | Removed duplicate, kept `AppUser2026Secure` |
| 3 | SSL cert validation fail | Changed `POSTGRES_HOST` to IP address, broke SSL hostname validation | ✅ Fixed | Restored hostname, used `extra_hosts` for IPv4 |
| 4 | Transaction rollback on login | **Column name mismatches** in SQL query (`password_hash` vs `password`, `first_name`/`last_name` vs `name`) | ✅ Fixed | Corrected column names in `getUserByEmail()` |
| 5 | RLS blocking queries | Initially suspected RLS, but verified RLS policies work correctly from psql | ✅ Not the issue | N/A |

---

## Final Resolution: Transaction Rollback Issue

### The Actual Problem

**Error:** `failed to commit transaction: commit unexpectedly resulted in rollback`

**Root Cause:** SQL query referenced non-existent columns:
- Query used `password_hash` but actual column is `password`
- Query used `COALESCE(first_name || ' ' || last_name, ...)` but actual column is just `name`

When the query failed inside the transaction, PostgreSQL marked the transaction as aborted. When pgx tried to commit, PostgreSQL rejected it with a rollback.

### The Fix

**File:** `go-backend/internal/handlers/auth.go` (lines 554-620)

**Changes:**
1. Bypassed `SuperAdminTransaction` for login queries (use direct queries instead)
2. Fixed column names:
   - `password_hash` → `password`
   - `COALESCE(first_name || ' ' || last_name, ...)` → `name`

**Before:**
```go
if err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
    query := `
        SELECT id, tenant_id, email, password_hash,
               COALESCE(first_name || ' ' || last_name, first_name, last_name, '') as name,
               role, status, failed_login_attempts, locked_until
        FROM users
        WHERE email = $1 AND deleted_at IS NULL
    `
    // ...
}); err != nil {
    return nil, err
}
```

**After:**
```go
// Bypass transaction for login - use direct query to avoid Neon pooler issues
if tenantID != nil {
    query := `
        SELECT id, tenant_id, email, password, name,
               role, status, failed_login_attempts, locked_until
        FROM users
        WHERE email = $1 AND tenant_id = $2 AND deleted_at IS NULL
    `
    qErr = h.db.QueryRow(ctx, query, email, *tenantID).Scan(...)
} else {
    query := `
        SELECT id, tenant_id, email, password, name,
               role, status, failed_login_attempts, locked_until
        FROM users
        WHERE email = $1 AND deleted_at IS NULL
    `
    qErr = h.db.QueryRow(ctx, query, email).Scan(...)
}
```

### Additional Fix: QueryExecMode

**File:** `go-backend/internal/database/postgres.go` (line 39)

Changed from `QueryExecModeExec` to `QueryExecModeSimpleProtocol` for better PgBouncer compatibility:

```go
// BEFORE
poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

// AFTER
poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
```

This aligns the code with the comment ("Use simple query mode for Neon's pgbouncer-based connection pooler") and uses the same protocol as psql.

---

## Verification

✅ Login works:
```bash
curl -X POST https://api.irislondonshoes.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test123!"}'
# Returns: access_token, refresh_token, user info
```

✅ Health check passes:
```bash
curl https://api.irislondonshoes.com/health
# {"status":"ok","goroutines":11,"memory_mb":2,"uptime":"12.7s"}
```

✅ Ready check passes:
```bash
curl https://api.irislondonshoes.com/ready
# {"db":"ok","redis":"ok","ai":"ok"}
```

---

## Key Learnings

1. **Transaction rollback errors can mask query errors** — When a query fails inside a transaction, PostgreSQL marks the transaction as aborted. The commit then fails with "commit unexpectedly resulted in rollback", hiding the original error.

2. **Check actual database schema** — The code was written assuming `first_name`/`last_name` columns existed, but the actual schema uses a single `name` column. Always verify against the live database.

3. **Use direct queries for read-only operations** — For login (which is read-only), wrapping in a transaction adds unnecessary complexity. Direct queries are simpler and easier to debug.

4. **Neon/PgBouncer compatibility** — While `QueryExecModeSimpleProtocol` wasn't the fix for this specific issue, it's still the correct setting for Neon's PgBouncer-based pooler and avoids potential edge cases with the extended query protocol.

---

## Deployment Checklist

- [x] Fixed column name mismatches in `auth.go`
- [x] Changed `QueryExecMode` to `SimpleProtocol`
- [x] Rebuilt Docker image
- [x] Deployed to production
- [x] Verified login works
- [x] Verified health/ready endpoints
- [x] No transaction rollback errors in logs

---

**Last Updated:** 2026-09-07  
**Status:** ✅ RESOLVED
