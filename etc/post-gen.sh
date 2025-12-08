#!/usr/bin/env bash
set -euo pipefail

for file in jsonschema/*.json; do
    [[ -e "$file" ]] || continue

    filename="$(basename "$file")"
    dir="$(dirname "$file")"

    # Extract the first 3 and the 4th dot-separated fields
    prefix=$(echo "$filename" | cut -d '.' -f 1-3)
    base=$(echo "$filename" | cut -d '.' -f 4)

    # Convert CamelCase/PascalCase to kebab-case
    kebab=$(echo "$base" \
        | sed 's/\([A-Z]\)/-\1/g' \
        | sed 's/^-//' \
        | tr 'A-Z' 'a-z')

    newname="${prefix}.${kebab}.json"
    newpath="${dir}/${newname}"

    echo "Renaming: $filename → $newname"
    mv "$file" "$newpath"
done
