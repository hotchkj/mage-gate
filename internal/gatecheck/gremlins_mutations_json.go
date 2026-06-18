// Vision: Single source of truth for decoding gremlins mutations.json (files vs flat layout).
package gatecheck

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// unknownFile is the canonical path key when gremlins omits file/filename on a mutation.
const unknownFile = "unknown"

// jsonNullRaw is the canonical JSON null spelling for RawMessage comparisons.
const jsonNullRaw = "null"

// Gremlins mutations.json parse contract:
//
// We extract only what we need and skip fields we don't understand. The parser fails only when
// the structural shape prevents iteration or a required field is missing/unusable:
//
//   - Top-level JSON must be a JSON object (otherwise nothing is extractable).
//   - If "files" is present and not null, it must be a JSON array (errGremlinsFilesNotArray).
//     Non-empty "files" is authoritative; root "mutations" is ignored.
//   - In authoritative "files" mode, every files[] entry must include a "mutations" JSON array.
//     Missing, null, or non-array "mutations" is a hard parse error
//     (errGremlinsFileMutationsNotArray).
//   - If "files" is absent, null, or empty, "mutations" is used; if present and not null it must
//     be a JSON array (errGremlinsMutationsNotArray).
//   - Non-string "file", "filename", or "package" values are silently treated as missing
//     (attributed to "unknown") — we do not control gremlins' choice of output.
//   - For kills: "status" must be a string (errMissingStatus). Unrecognized status text fails
//     with errUnknownStatus. For sites: "status" is not inspected.

// parseGremlinsMutationRoot decodes the top-level gremlins JSON object.
func parseGremlinsMutationRoot(data []byte) (map[string]json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse gremlins mutations JSON: %w", err)
	}
	return root, nil
}

// selectGremlinsFilesRaw returns filesRaw and useFiles=true when "files" is a non-empty JSON array.
// If "files" is present but not a JSON array, it returns a non-nil error (no silent fallback to "mutations").
func selectGremlinsFilesRaw(root map[string]json.RawMessage) (filesRaw json.RawMessage, useFiles bool, err error) {
	filesRaw, ok := root["files"]
	if !ok {
		return nil, false, nil
	}
	if len(filesRaw) == 0 || string(filesRaw) == jsonNullRaw {
		return nil, false, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(filesRaw, &entries); err != nil {
		return nil, false, fmt.Errorf("%w: %w", errGremlinsFilesNotArray, err)
	}
	if len(entries) == 0 {
		return nil, false, nil
	}
	return filesRaw, true, nil
}

// gremlinsFileBundle is one files[] entry after resolving paths and unmarshaling mutations.
type gremlinsFileBundle struct {
	File      string
	Package   string
	Mutations []map[string]any
}

// parseGremlinsFileBundles unmarshals the files section once for both site counts and kill stats.
func parseGremlinsFileBundles(filesRaw json.RawMessage) ([]gremlinsFileBundle, error) {
	var files []struct {
		FileName  string          `json:"file_name"`
		Filename  string          `json:"filename"`
		Package   string          `json:"package"`
		Mutations json.RawMessage `json:"mutations"`
	}
	if err := json.Unmarshal(filesRaw, &files); err != nil {
		return nil, fmt.Errorf("parse gremlins files: %w", err)
	}

	bundles := make([]gremlinsFileBundle, 0, len(files))
	for _, entry := range files {
		file := pickFileName(entry.FileName, entry.Filename)
		pkg := normalizePackageName(entry.Package)
		if len(entry.Mutations) == 0 || string(entry.Mutations) == jsonNullRaw {
			return nil, fmt.Errorf("parse mutations for %q: %w", file, errGremlinsFileMutationsNotArray)
		}
		var muts []map[string]any
		if err := json.Unmarshal(entry.Mutations, &muts); err != nil {
			return nil, fmt.Errorf("parse mutations for %q: %w: %w", file, errGremlinsFileMutationsNotArray, err)
		}
		bundles = append(bundles, gremlinsFileBundle{
			File:      file,
			Package:   pkg,
			Mutations: muts,
		})
	}
	return bundles, nil
}

// parseGremlinsFlatMutationMaps decodes root-level "mutations" when the files branch is not used.
// If "mutations" is present and not null, it MUST be a JSON array.
func parseGremlinsFlatMutationMaps(root map[string]json.RawMessage) ([]map[string]any, error) {
	mutRaw, ok := root["mutations"]
	if !ok {
		return nil, nil
	}
	if len(mutRaw) == 0 || string(mutRaw) == jsonNullRaw {
		return nil, nil
	}
	var mutations []map[string]any
	if err := json.Unmarshal(mutRaw, &mutations); err != nil {
		return nil, fmt.Errorf("%w: %w", errGremlinsMutationsNotArray, err)
	}
	return mutations, nil
}

func pickFileName(primary, fallback string) string {
	if primary != "" {
		return fsnorm.Canonical(primary)
	}
	if fallback != "" {
		return fsnorm.Canonical(fallback)
	}
	return unknownFile
}

func stringField(value map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok {
			continue
		}
		text, ok := raw.(string)
		if ok {
			return text
		}
	}
	return ""
}

// firstNonEmptyStringField returns the first key whose value is a non-empty string after trim.
// Gremlins may set "file" to "" while populating "filename"; treat empty strings as missing.
func firstNonEmptyStringField(value map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok {
			continue
		}
		text, ok := raw.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		return text
	}
	return ""
}

// countMutationSitesFromRoot counts mutation entries per file using the same layout rules as kill parsing.
func countMutationSitesFromRoot(root map[string]json.RawMessage) (map[string]int, error) {
	filesRaw, useFiles, err := selectGremlinsFilesRaw(root)
	if err != nil {
		return nil, err
	}
	if useFiles {
		bundles, bundleErr := parseGremlinsFileBundles(filesRaw)
		if bundleErr != nil {
			return nil, bundleErr
		}
		out := map[string]int{}
		for _, b := range bundles {
			out[b.File] += len(b.Mutations)
		}
		return out, nil
	}
	muts, err := parseGremlinsFlatMutationMaps(root)
	if err != nil {
		return nil, err
	}
	return flatMutationSiteCounts(muts), nil
}

func flatMutationSiteCounts(muts []map[string]any) map[string]int {
	out := map[string]int{}
	for _, mutation := range muts {
		filePath := firstNonEmptyStringField(mutation, "file", "filename")
		if filePath == "" {
			filePath = unknownFile
		}
		out[fsnorm.Canonical(filePath)]++
	}
	return out
}
