package schema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// InlineBundledSchemasInFS finds all *.json files in fsys, and for each file:
// - parses JSON
// - inlines local $ref pointers like "#/$defs/..."
// - removes $defs (everywhere)
// - removes all $id (everywhere, including top-level)
// - removes all $schema except the top-level $schema
// - pretty-prints the result
//
// Returns a map of updated file contents keyed by file path.
// If fsys is writable, it will also write each updated file back to fsys.
func InlineBundledSchemasInFS(fsys fs.FS) (map[string][]byte, error) {
	updates := map[string][]byte{}

	// Optional write-back support for writable FS implementations.
	type writeFileFS interface {
		WriteFile(name string, data []byte, perm fs.FileMode) error
	}
	var writer writeFileFS
	if w, ok := fsys.(writeFileFS); ok {
		writer = w
	}

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		b, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var root any
		if err := json.Unmarshal(b, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		// Inline refs using the original root (which still includes $defs).
		resolved, err := inlineRefs(root, root, nil)
		if err != nil {
			return fmt.Errorf("inline refs in %s: %w", path, err)
		}

		// Cleanup:
		// - remove all $id everywhere
		// - remove all $schema except top-level
		// - remove all $defs everywhere
		resolved = stripKeys(resolved, true /*keepTopLevelSchema*/)

		out, err := json.MarshalIndent(resolved, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", path, err)
		}
		out = append(out, '\n')

		updates[filepath.ToSlash(path)] = out

		// Write back if possible
		if writer != nil {
			info, statErr := fs.Stat(fsys, path)
			perm := fs.FileMode(0644)
			if statErr == nil {
				perm = info.Mode().Perm()
			}
			if err := writer.WriteFile(path, out, perm); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return updates, nil
}

func inlineRefs(node any, root any, stack []string) (any, error) {
	switch v := node.(type) {
	case map[string]any:
		// If this object has a $ref, inline it (local refs only).
		if refVal, ok := v["$ref"]; ok {
			refStr, ok := refVal.(string)
			if !ok {
				return nil, fmt.Errorf("$ref must be a string, got %T", refVal)
			}
			if contains(stack, refStr) {
				return nil, fmt.Errorf("cyclic $ref detected: %s", strings.Join(append(stack, refStr), " -> "))
			}

			target, err := getByPointer(root, refStr)
			if err != nil {
				return nil, err
			}

			// Resolve the target first.
			resolvedTarget, err := inlineRefs(deepClone(target), root, append(stack, refStr))
			if err != nil {
				return nil, err
			}

			// Resolve siblings (everything except $ref and $defs) and merge (siblings win).
			siblings := make(map[string]any, len(v))
			for k, child := range v {
				if k == "$ref" || k == "$defs" {
					continue
				}
				resolvedChild, err := inlineRefs(child, root, stack)
				if err != nil {
					return nil, err
				}
				siblings[k] = resolvedChild
			}

			// Merge if both are objects.
			if rm, ok := resolvedTarget.(map[string]any); ok {
				out := make(map[string]any, len(rm)+len(siblings))
				for k, val := range rm {
					if k == "$defs" {
						continue
					}
					out[k] = val
				}
				for k, val := range siblings {
					out[k] = val
				}
				return out, nil
			}

			// If resolved target isn't an object, return it (siblings can't reliably merge).
			return resolvedTarget, nil
		}

		// Normal object: recursively resolve all keys, skipping "$defs".
		out := make(map[string]any, len(v))
		for k, child := range v {
			if k == "$defs" {
				continue
			}
			resolvedChild, err := inlineRefs(child, root, stack)
			if err != nil {
				return nil, err
			}
			out[k] = resolvedChild
		}
		return out, nil

	case []any:
		out := make([]any, len(v))
		for i := range v {
			r, err := inlineRefs(v[i], root, stack)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil

	default:
		return node, nil
	}
}

// stripKeys removes:
// - all "$id" fields everywhere
// - all "$schema" fields except top-level (if keepTopLevelSchema==true)
// - all "$defs" fields everywhere
func stripKeys(node any, keepTopLevelSchema bool) any {
	// Capture the original top-level $schema if we want to preserve it.
	var topSchema any
	if keepTopLevelSchema {
		if m, ok := node.(map[string]any); ok {
			if v, exists := m["$schema"]; exists {
				topSchema = v
			}
		}
	}

	cleaned := stripKeysRecursive(node)

	// Restore top-level $schema only (if it existed).
	if keepTopLevelSchema && topSchema != nil {
		if m, ok := cleaned.(map[string]any); ok {
			m["$schema"] = topSchema
		}
	}

	return cleaned
}

func stripKeysRecursive(node any) any {
	switch v := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			// Remove everywhere:
			if k == "$id" || k == "$defs" || k == "$schema" {
				continue
			}
			out[k] = stripKeysRecursive(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = stripKeysRecursive(v[i])
		}
		return out
	default:
		return node
	}
}

// getByPointer resolves a local JSON Pointer against root.
// Supports pointers like "#/a/b" (commonly "#/$defs/Name").
// Implements JSON Pointer unescaping: ~1 => /, ~0 => ~
func getByPointer(root any, ptr string) (any, error) {
	if !strings.HasPrefix(ptr, "#/") {
		return nil, fmt.Errorf("only local refs supported, got: %q", ptr)
	}

	path := strings.TrimPrefix(ptr, "#/")
	if path == "" {
		return root, nil
	}
	parts := strings.Split(path, "/")

	cur := root
	for _, raw := range parts {
		p := strings.ReplaceAll(raw, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")

		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("pointer %q: encountered non-object at %q (got %T)", ptr, p, cur)
		}
		next, ok := obj[p]
		if !ok {
			return nil, fmt.Errorf("unresolved $ref %q: missing key %q", ptr, p)
		}
		cur = next
	}
	return cur, nil
}

func contains(stack []string, s string) bool {
	for _, x := range stack {
		if x == s {
			return true
		}
	}
	return false
}

func deepClone(v any) any {
	// JSON round-trip clone (fine for schema-sized objects).
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
