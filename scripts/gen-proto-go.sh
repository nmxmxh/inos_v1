#!/usr/bin/env bash
set -euo pipefail

# Generic Cap'n Proto Go Code Generator (Strictly Agnostic)
# Automatically discovers and processes all .capnp schemas without polluting source files

SCHEMA_ROOT="protocols/schemas"
OUT_DIR="${1:-kernel/gen}"

echo "🔧 Generating Go bindings for all Cap'n Proto schemas..."
mkdir -p "$OUT_DIR"

# Create temporary go.capnp for annotations
mkdir -p "$SCHEMA_ROOT/.tmp"
cat > "$SCHEMA_ROOT/.tmp/go.capnp" << 'EOF'
@0xd12a1c51fedd6c88;

annotation package(file) :Text;
annotation import(file) :Text;
EOF

# Step 1: Create all transient Go-annotated files
temp_files=()
find "$SCHEMA_ROOT" -name "*.capnp" -type f -not -path "*/.tmp/*" | while read -r schema_path; do
    rel_path="${schema_path#$SCHEMA_ROOT/}"
    pkg_name=$(dirname "$rel_path" | cut -d'/' -f1)
    file_id=$(head -n 1 "$schema_path")
    
    temp_file="${schema_path%.capnp}-go-temp.capnp"
    {
        echo "$file_id"
        echo ""
        echo "using Go = import \"/.tmp/go.capnp\";"
        echo ""
        echo "\$Go.package(\"$pkg_name\");"
        # Use full versioned path for imports
        version_dir=$(dirname "$rel_path" | cut -d'/' -f2)
        echo "\$Go.import(\"github.com/nmxmxh/inos_v1/$OUT_DIR/$pkg_name/$version_dir\");"
        echo ""
        # Rewrite imports to point to transient Go-annotated files
        tail -n +2 "$schema_path" | sed 's/import "\([^"]*\)\.capnp"/import "\1-go-temp.capnp"/g'
    } > "$temp_file"
    
    echo "$temp_file" >> "$SCHEMA_ROOT/.tmp/files.txt"
done

# Step 2: Compile all transient files together
while IFS= read -r temp_file; do
    rel_path="${temp_file#$SCHEMA_ROOT/}"
    rel_path="${rel_path%-go-temp.capnp}.capnp"
    pkg_name=$(dirname "$rel_path" | cut -d'/' -f1)
    
    echo "  Processing: $rel_path → $pkg_name"
    mkdir -p "$OUT_DIR/$pkg_name"
    
    # Compile with output going directly to package directory
    if ! capnp compile -I"$SCHEMA_ROOT" "--src-prefix=$SCHEMA_ROOT" -ogo:"$OUT_DIR" "$temp_file"; then
        echo "❌ Failed to compile $rel_path"
        # Cleanup all temp files
        while IFS= read -r f; do rm -f "$f"; done < "$SCHEMA_ROOT/.tmp/files.txt"
        rm -rf "$SCHEMA_ROOT/.tmp"
        exit 1
    fi
done < "$SCHEMA_ROOT/.tmp/files.txt"

# Step 3: Cleanup temp files and fix generated file names
while IFS= read -r temp_file; do
    # Remove temp schema file
    rm -f "$temp_file"
    
    # Fix generated Go file naming (remove -go-temp from filenames)
    rel_path="${temp_file#$SCHEMA_ROOT/}"
    pkg_name=$(dirname "$rel_path" | cut -d'/' -f1)
    base_name=$(basename "$rel_path" -go-temp.capnp)
    
    # Find and rename the generated file
    find "$OUT_DIR" -name "*${base_name}-go-temp.capnp.go" -exec bash -c '
        for file; do
            new_name=$(echo "$file" | sed "s/-go-temp\.capnp\.go$/.capnp.go/")
            mv "$file" "$new_name"
        done
    ' bash {} +
done < "$SCHEMA_ROOT/.tmp/files.txt"

# Remove any nested protocols/schemas directories that capnp might have created
if [ -d "$OUT_DIR/protocols" ]; then
    # Move files from nested structure to correct package directories
    find "$OUT_DIR/protocols/schemas" -name "*.go" 2>/dev/null | while read -r nested_file; do
        rel_to_schemas="${nested_file#$OUT_DIR/protocols/schemas/}"
        pkg_name=$(dirname "$rel_to_schemas" | cut -d'/' -f1)
        base_name=$(basename "$rel_to_schemas")
        mkdir -p "$OUT_DIR/$pkg_name"
        mv "$nested_file" "$OUT_DIR/$pkg_name/$base_name"
    done
    rm -rf "$OUT_DIR/protocols"
fi

rm -rf "$SCHEMA_ROOT/.tmp"

echo "✅ Go protocol generation complete in $OUT_DIR"
