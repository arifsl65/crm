#!/bin/bash
# =============================================================================
# Accountant CRM - Full Codebase Sync Script
# =============================================================================
# Regenerates Final_md/FULL_CODEBASE.md with:
#   1. Directory tree structure
#   2. All source code files (264+ files)
#   3. PostgreSQL schema + sample data (38 tables)
#   4. MongoDB schema + sample data
#
# Usage: ./scripts/sync-codebase-md.sh
# =============================================================================

set -e

cd "$(dirname "$0")/.."
ROOT_DIR=$(pwd)
OUTPUT="Final_md/FULL_CODEBASE.md"
TIMESTAMP=$(date -Iseconds)

echo "=========================================="
echo "Syncing FULL_CODEBASE.md"
echo "=========================================="

# Load environment variables
if [ -f .env ]; then
    export $(grep -v '^#' .env | grep -v '^$' | xargs)
    echo "✓ Loaded .env"
else
    echo "⚠ No .env file found - database sections will be skipped"
fi

# Create output directory if needed
mkdir -p Final_md

# =============================================================================
# SECTION 1: Header + Directory Tree
# =============================================================================
echo "→ Generating directory tree..."

cat > "$OUTPUT" << EOF
# Accountant CRM - Full Codebase

**Generated:** $TIMESTAMP

> **To regenerate:** \`./scripts/sync-codebase-md.sh\`

Contents:
- Directory tree
- Source code (Go, Python, TypeScript, SQL)
- PostgreSQL schema + sample data (38 tables)
- MongoDB schema + sample data

---

## Directory Structure

\`\`\`
EOF

tree -I 'node_modules|.next|.next-old|__pycache__|.venv|.git|playwright-report|test-results|tmp|.terraform|Final_md|out|bin' \
    --dirsfirst 2>/dev/null >> "$OUTPUT" || echo "(tree command not available)" >> "$OUTPUT"

echo '```' >> "$OUTPUT"
echo "" >> "$OUTPUT"
echo "---" >> "$OUTPUT"
echo "" >> "$OUTPUT"

# =============================================================================
# SECTION 2: Source Code Files
# =============================================================================
echo "→ Processing source files..."

# Function to get language for code fence
get_lang() {
    case "$1" in
        *.go) echo "go" ;;
        *.py) echo "python" ;;
        *.ts|*.tsx) echo "typescript" ;;
        *.js|*.jsx) echo "javascript" ;;
        *.sql) echo "sql" ;;
        *.json) echo "json" ;;
        *.yaml|*.yml) echo "yaml" ;;
        *.toml) echo "toml" ;;
        *.md) echo "markdown" ;;
        *.sh) echo "bash" ;;
        *.graphql|*.graphqls|*.gql) echo "graphql" ;;
        *.css) echo "css" ;;
        *Dockerfile*) echo "dockerfile" ;;
        *) echo "" ;;
    esac
}

# Find and process source files
FILES_LIST=$(mktemp)
find . -type f \
    \( -name "*.go" -o -name "*.py" -o -name "*.ts" -o -name "*.tsx" \
       -o -name "*.js" -o -name "*.jsx" -o -name "*.sql" -o -name "*.json" \
       -o -name "*.yaml" -o -name "*.yml" -o -name "*.toml" -o -name "*.sh" \
       -o -name "*.graphql" -o -name "*.graphqls" -o -name "*.css" \
       -o -name "Dockerfile*" -o -name "docker-compose*.yml" \
       -o -name ".env.example" -o -name "*.mod" \) \
    -not -path "./.git/*" \
    -not -path "./node_modules/*" \
    -not -path "./frontend/node_modules/*" \
    -not -path "./e2e/node_modules/*" \
    -not -path "./frontend/.next/*" \
    -not -path "./frontend/.next-old/*" \
    -not -path "./frontend/out/*" \
    -not -path "./__pycache__/*" \
    -not -path "./python-ai/__pycache__/*" \
    -not -path "./python-ai/.venv/*" \
    -not -path "./go-backend/tmp/*" \
    -not -path "./Final_md/*" \
    -not -path "./.terraform/*" \
    -not -path "./e2e/playwright-report/*" \
    -not -path "./e2e/test-results/*" \
    -not -name "package-lock.json" \
    -not -name "pnpm-lock.yaml" \
    -not -name "yarn.lock" \
    -not -name "go.sum" \
    -not -name "generated.go" \
    -not -name "*.gen.go" \
    -not -name "*.tfstate*" \
    -not -name "tsconfig.tsbuildinfo" \
    | sort > "$FILES_LIST"

FILE_COUNT=0
while IFS= read -r file; do
    # Skip files larger than 500KB
    size=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null || echo "0")
    if [ "$size" -gt 512000 ]; then
        echo "  Skipping large file: $file ($size bytes)" >&2
        continue
    fi

    # Skip binary files
    if file "$file" 2>/dev/null | grep -q "executable\|binary\|ELF"; then
        continue
    fi

    relpath="${file#./}"
    lang=$(get_lang "$file")

    echo "## $relpath" >> "$OUTPUT"
    echo "" >> "$OUTPUT"
    echo "\`\`\`$lang" >> "$OUTPUT"
    cat "$file" >> "$OUTPUT"
    echo "" >> "$OUTPUT"
    echo "\`\`\`" >> "$OUTPUT"
    echo "" >> "$OUTPUT"
    echo "---" >> "$OUTPUT"
    echo "" >> "$OUTPUT"

    FILE_COUNT=$((FILE_COUNT + 1))
done < "$FILES_LIST"

rm -f "$FILES_LIST"
echo "  Processed $FILE_COUNT source files"

# =============================================================================
# SECTION 3: PostgreSQL Schema + Sample Data
# =============================================================================
if [ -n "$POSTGRES_HOST" ] && [ -n "$POSTGRES_PASSWORD" ]; then
    echo "→ Dumping PostgreSQL schema..."

    DB_URL="postgresql://${POSTGRES_USER}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=require"

    cat >> "$OUTPUT" << EOF
## PostgreSQL Schema with Sample Data

**Database:** \`$POSTGRES_DB\` @ \`$POSTGRES_HOST\`
**Generated:** $TIMESTAMP

EOF

    # Get tables (excluding partitions)
    TABLES=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql "$DB_URL" -t -c "
        SELECT tablename FROM pg_tables
        WHERE schemaname = 'public'
        AND tablename NOT LIKE '%_202%'
        ORDER BY tablename;
    " 2>/dev/null)

    TABLE_COUNT=0
    for table in $TABLES; do
        echo "  Processing table: $table" >&2

        echo "### Table: \`$table\`" >> "$OUTPUT"
        echo "" >> "$OUTPUT"

        # Schema
        echo "**Schema:**" >> "$OUTPUT"
        echo '```sql' >> "$OUTPUT"
        PGPASSWORD="${POSTGRES_PASSWORD}" psql "$DB_URL" -c "\d $table" 2>/dev/null | head -50 >> "$OUTPUT"
        echo '```' >> "$OUTPUT"
        echo "" >> "$OUTPUT"

        # Row count
        COUNT=$(PGPASSWORD="${POSTGRES_PASSWORD}" psql "$DB_URL" -t -c "SELECT COUNT(*) FROM $table;" 2>/dev/null | tr -d ' ')
        echo "**Rows:** $COUNT" >> "$OUTPUT"
        echo "" >> "$OUTPUT"

        # Sample data (3 rows, masked)
        if [ "$COUNT" != "0" ] && [ -n "$COUNT" ]; then
            echo "**Sample (3 rows):**" >> "$OUTPUT"
            echo '```' >> "$OUTPUT"
            PGPASSWORD="${POSTGRES_PASSWORD}" psql "$DB_URL" -x -c "SELECT * FROM $table LIMIT 3;" 2>/dev/null | \
                sed -e 's/\$argon2[^|]*/***HASHED***/g' \
                    -e 's/eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*/***JWT***/g' \
                    -e 's/\(password[^|]*|\s*\)[^|]*/\1***MASKED***/gi' \
                    -e 's/\(secret[^|]*|\s*\)[^|]*/\1***MASKED***/gi' \
                    -e 's/\(api_key[^|]*|\s*\)[A-Za-z0-9]\{20,\}/\1***MASKED***/gi' \
                    -e 's/\(token[^|]*|\s*\)[A-Za-z0-9_-]\{40,\}/\1***MASKED***/gi' \
                >> "$OUTPUT"
            echo '```' >> "$OUTPUT"
        fi

        echo "" >> "$OUTPUT"
        echo "---" >> "$OUTPUT"
        echo "" >> "$OUTPUT"

        TABLE_COUNT=$((TABLE_COUNT + 1))
    done

    echo "  Processed $TABLE_COUNT PostgreSQL tables"
else
    echo "⚠ Skipping PostgreSQL (no credentials)"
fi

# =============================================================================
# SECTION 4: MongoDB Schema + Sample Data
# =============================================================================
if [ -n "$MONGODB_URI" ]; then
    echo "→ Dumping MongoDB schema..."

    cat >> "$OUTPUT" << EOF
## MongoDB Schema with Sample Data

**Database:** \`accountant_ai\` (MongoDB Atlas)
**Generated:** $TIMESTAMP

EOF

    mongosh "$MONGODB_URI" --quiet --eval '
    db = db.getSiblingDB("accountant_ai");
    const collections = db.getCollectionNames();

    collections.forEach(coll => {
        print("### Collection: `" + coll + "`\n");

        const count = db[coll].countDocuments();
        print("**Documents:** " + count + "\n");

        if (count > 0) {
            print("**Sample Document:**");
            print("```json");
            const sample = db[coll].findOne();

            // Truncate large arrays/strings
            if (sample && sample.messages && sample.messages.length > 2) {
                sample.messages = sample.messages.slice(0, 2);
                sample.messages.push({ "_note": "...truncated..." });
            }

            // Mask sensitive fields
            ["api_key", "token", "password", "secret"].forEach(key => {
                if (sample && sample[key]) sample[key] = "***MASKED***";
            });

            print(JSON.stringify(sample, null, 2));
            print("```\n");
        }

        print("**Indexes:**");
        print("```");
        db[coll].getIndexes().forEach(idx => {
            print(idx.name + ": " + JSON.stringify(idx.key));
        });
        print("```\n");
        print("---\n");
    });
    ' 2>/dev/null >> "$OUTPUT" || echo "⚠ MongoDB connection failed" >&2

    echo "  Processed MongoDB collections"
else
    echo "⚠ Skipping MongoDB (no credentials)"
fi

# =============================================================================
# Summary
# =============================================================================
LINES=$(wc -l < "$OUTPUT")
SIZE=$(du -h "$OUTPUT" | cut -f1)
SECTIONS=$(grep -c "^## " "$OUTPUT" || echo "0")

echo ""
echo "=========================================="
echo "✓ Sync Complete!"
echo "=========================================="
echo "Output:   $OUTPUT"
echo "Lines:    $LINES"
echo "Size:     $SIZE"
echo "Sections: $SECTIONS"
echo "=========================================="
