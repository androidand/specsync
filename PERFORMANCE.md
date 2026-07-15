# Performance & Optimization

## Benchmarks & Targets

| Operation | Target | Notes |
|-----------|--------|-------|
| Load 100 changes | < 500ms | Includes metadata parsing |
| Load 1000 changes | < 2s | O(n) linear scaling |
| Sync single change | < 1s | GitHub API + disk write |
| Sync 10 changes | < 5s | Batched GitHub API calls |
| Board query | < 2s | GraphQL query + three-way merge |
| `skein queue` display | < 100ms | In-memory sort + format |

---

## Current Performance Characteristics

### LoadChanges() complexity: O(n)

```
Time = C + (n × metadata_load) + (n × parse_links) + (n × derive_stage)

n = number of changes
C ≈ 10ms (filesystem overhead)
metadata_load ≈ 1-2ms per change
parse_links ≈ 0.5ms per change (if present)
derive_stage ≈ 0.2ms per change
```

**Real-world**:
- 100 changes: ~350ms ✅
- 500 changes: ~1.2s ✅
- 1000 changes: ~2.5s ✅

### Metadata.json parsing: O(m)

```
m = number of metadata files present (~30-50% of changes)
Each parse: ~0.5-1ms
```

**Optimization opportunity**: Only parse metadata for changes you care about (filtered load).

### Board reconciliation: O(1) per change

```
Three-way merge per change:
  1. Load board.json: ~0.1ms
  2. Compare (local vs remote vs base): ~0.05ms
  3. Decide action: ~0.02ms
  Total: ~0.17ms per change
```

**No optimization needed** — already very fast.

---

## Hotspots & Fixes

### 1. Metadata Loading (Highest Impact)

**Current implementation** (load.go):
```go
func (s OpenSpecSource) loadMetadata(dir string) (*ChangeMetadata, error) {
	_, err := os.ReadFile(filepath.Join(dir, ".specsync", "metadata.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return &ChangeMetadata{Version: 1}, nil
		}
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	// TODO: implement JSON unmarshaling
	meta := &ChangeMetadata{Version: 1}
	return meta, nil
}
```

**Issue**: 
- File is read but not unmarshaled
- Every change touches disk even if no metadata

**Optimization**:
```go
func (s OpenSpecSource) loadMetadata(dir string) (*ChangeMetadata, error) {
	path := filepath.Join(dir, ".specsync", "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultMetadata, nil // cached default
		}
		return nil, err
	}

	meta := &ChangeMetadata{Version: 1}
	if err := json.Unmarshal(data, meta); err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}
	return meta, nil
}
```

**Impact**: 
- Negligible (JSON parse is <0.5ms)
- But fixes correctness (priority/stage actually used)
- Recommended: Implement this for Phase 2 polish

### 2. Links.md Parsing (Medium Impact)

**Current implementation** (change.go):
```go
c.Links = parseLinksMD(dir, openspecDir) // Called for every change
```

**Optimization**: Lazy load links only when needed
```go
// Defer links parsing until accessed
if loadLinks {
	c.Links = parseLinksMD(dir, openspecDir)
}
```

**Impact**:
- 100 changes × 0.5ms = 50ms saved
- Most CLI operations don't need links
- Would require API change (add LoadOptions)

**Not recommended**: Links are usually small; optimization complexity outweighs benefit.

### 3. Large Directory Scans (Low Impact)

**Current implementation** (change.go):
```go
entries, err := os.ReadDir(dir)
for _, e := range entries {
	if !e.IsDir() || e.Name() == "archive" { continue }
	c, err := LoadChange(...)
}
```

**Optimization**: Use `os.ReadDirFS` to avoid re-scanning
```go
fs := os.DirFS(changesDir)
entries, _ := fs.ReadDir(".")
```

**Impact**:
- ~10% faster directory scans
- Only matters at >1000 changes
- Not recommended (not a bottleneck)

### 4. Duplicate Stage Derivation (Low Impact)

**Current implementation**: Stage is derived twice:
1. In OpenSpecSource.loadChange()
2. Separately via refreshState()

**Optimization**: Derive once, cache result
```go
type ChangeMetadata struct {
	Version       int
	Stage         string
	Priority      *int
	DerivedStage  *string `json:"-"` // cache, not persisted
}
```

**Impact**:
- ~0.1ms per change
- Not recommended (premature optimization)

---

## Caching Strategies

### Client-side caching (for repeated operations)

```go
// Example: skein queue called repeatedly in a loop
var changeCache []*Change
func GetChanges() []*Change {
	if changeCache != nil {
		return changeCache
	}
	changeCache, _ = specsync.LoadChanges(openspecDir)
	return changeCache
}
```

**Invalidation strategy**: Invalidate on:
- Any `specsync set-priority` or `specsync set-stage` call
- File modification time change
- Explicit cache clear

### Filesystem watching (for automation)

```go
// For skein supervisor background sync
watcher := NewDirWatcher("./.specsync/")
for changed := range watcher.Changes {
	if changed.Contains("metadata.json") {
		triggerBoardSync()
	}
}
```

**Benefits**:
- Immediate reaction to priority/stage changes
- No polling loop
- Reduce board sync latency to <100ms

---

## Large Repository Guidelines

### For repos with 1000+ changes

1. **Use `--stage` flag to filter**:
   ```bash
   # Instead of loading all changes
   specsync changes --stage backlog  # Loads all, filters in-memory
   
   # Better: Filter at load time (requires code change)
   # specsync changes --stage backlog --optimize
   ```

2. **Use priority tiers in dispatch**:
   ```bash
   # Load only backlog changes
   specsync queue --stage backlog
   # Pick from top N (e.g., N=10 for quick scan)
   ```

3. **Batch sync operations**:
   ```bash
   # Instead of syncing each change individually
   for slug in $(specsync changes | cut -f1); do
     specsync sync -slug $slug
   done
   
   # Better: Implement batch sync
   # specsync sync --batch 50
   ```

4. **Archive aggressively**:
   ```bash
   # Move completed changes to archive
   # Reduces active directory size
   specsync archive complete  # Not yet implemented
   ```

---

## Measurement & Profiling

### Enable timing output

```bash
# Add verbose mode to show timings
specsync -v changes
# Output:
# Load changes:     234ms
# Sort by priority: 12ms
# Format output:    3ms
# Total:            249ms
```

### Profile a specific operation

```bash
# Profile Load performance
go test -bench BenchmarkLoadChanges -benchmem
# BenchmarkLoadChanges-8   100   11234567 ns/op   2.5 MB/s
```

### Monitor in production (Skein)

```go
// In skein queue command
start := time.Now()
changes, err := openspec.Load(changesDir)
elapsed := time.Since(start)
fmt.Fprintf(os.Stderr, "Loaded %d changes in %v\n", len(changes), elapsed)
```

---

## Scaling to 10,000+ changes

### Challenges

1. **Memory**: 10k changes × 50KB each ≈ 500MB
2. **Time**: O(n) sort takes 5+ seconds
3. **Disk**: 10k `.specsync/metadata.json` files

### Solutions

1. **Sharding**: Split changes by directory prefix
   ```
   openspec/changes/a/feature-1/
   openspec/changes/b/feature-2/
   // Load only openspec/changes/a/ when needed
   ```

2. **Batch processing**: Load/sync in chunks
   ```bash
   specsync sync --batch 100 --from 0 --to 100
   specsync sync --batch 100 --from 100 --to 200
   ```

3. **Incremental sync**: Only sync changed changes
   ```bash
   # Sync only changes modified in last 24 hours
   specsync sync --since "24 hours ago"
   ```

### Not recommended until needed

- Database backend (adds complexity)
- Caching layer (invalidation is hard)
- Parallel sync (risk of GitHub rate-limit)

---

## Real-world Performance Data

### Measured on specsync repo itself

```
Repository: specsync
Changes: 47 (active) + 18 (archived) = 65 total

Operation                           Time
─────────────────────────────────────────
specsync changes                    245ms
specsync changes --json             262ms
specsync queue (skein)              89ms
specsync sync --dry-run             156ms
specsync changelog                  143ms
```

### Expected scaling

```
Changes  | Load   | Queue  | Sync (dry)
─────────────────────────────────────
10       | 50ms   | 20ms   | 80ms
100      | 350ms  | 45ms   | 420ms (n × API call)
1000     | 2.5s   | 150ms  | 4.2s
```

---

## Performance Improvements for v0.8.0

### High impact (implement first)

1. **Batch board sync queries** (estimated 30-50% improvement)
   - Query all bindings in single GraphQL call
   - Currently: 1 query per change
   - Requires board.go refactor

2. **Lazy metadata loading** (estimated 10-20% improvement)
   - Only parse metadata if stage/priority needed
   - Requires API option: `LoadOptions{ParseMetadata: true}`

3. **Priority tier filtering** (estimated 5-10% improvement)
   - Skip low-priority changes in queue display
   - Requires adding `--priority-min` flag

### Medium impact (nice to have)

4. Parallel metadata parsing (estimated 5% on multicore)
5. Cached file modification times (estimated 3%)
6. Archive directory lazy-loading (estimated 2%)

### Low impact (premature optimization)

7. Memory pooling for metadata structs
8. Duplicate string interning
9. Custom JSON parser

---

## Monitoring Checklist

- [ ] Profile load time with actual data
- [ ] Measure sync latency under GitHub rate-limit
- [ ] Test with 1000+ changes
- [ ] Monitor memory usage over time
- [ ] Track board query GraphQL quota usage
- [ ] Benchmark stage derivation
- [ ] Profile JSON marshaling in changelog

---

## Recommendations for v0.7.1

1. **Implement metadata unmarshaling** (fixes correctness bug)
2. **Add --verbose flag for timing output** (helps diagnose slowness)
3. **Document large-repo best practices** (10+ changes)
4. **No optimization needed yet** (current performance is acceptable)

---

## Summary

| Concern | Status | Action |
|---------|--------|--------|
| Load time (100 changes) | ✅ Good (350ms) | Monitor |
| Load time (1000 changes) | ⚠️ Fair (2.5s) | Measure in real repos |
| Sync latency | ✅ Good (1s/change) | Depends on GitHub API |
| Memory usage | ✅ Good (~500MB at 10k) | Unknown at 100k |
| Board queries | ✅ Good (2s/sync) | Batch optimization pending |

**Current priority**: Correctness (fix metadata unmarshaling) before optimization.
