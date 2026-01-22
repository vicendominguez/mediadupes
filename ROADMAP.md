# Roadmap

## Current Status

### ✅ Implemented Features

- **Plan mode** (`--plan`) - Shows space savings estimate without copying
- **Detailed statistics** - Files, types, metadata, performance metrics
- **Exclude patterns** (`--exclude`) - Filter files/directories with override behavior
- **Extension filtering** - Custom image and video extensions
- **Permissions preservation** - Timestamps preserved for EXIF files
- **Progress tracking** - Real-time progress bar with worker count and time
- **Version flag** (`--version`, `-v`) - Show tool version
- **Configurable concurrency** (`--workers`) - Adjust parallel processing
- **Metadata prioritization** - Keeps files with EXIF data over those without
- **Recursive scanning** - With `--no-recursive` option to disable
- **Directory tree preservation** - Maintains original folder structure in destination
- **One-dir mode** (`--one-dir`) - Flatten all files to destination root
- **Verbose mode** (`--verbose`, `-V`) - Real-time file processing display with toggle
- **Error tracking** - Real-time error box showing last 10 failures with counts
- **Parallel EXIF processing** - One exiftool instance per worker for true concurrency

## Planned Changes

### 🚧 In Progress - Directory Structure Overhaul

#### Phase 1: Preserve Directory Tree (Breaking Change)
**Status:** Planned  
**Priority:** High

Change default behavior to preserve source directory structure in destination.

**Changes:**
- Add `RelPath` field to `FileInfo` struct
- Modify `walkFiles()` to calculate relative paths from source root
- Update `worker()` to store relative path in `FileInfo`
- Rewrite `copyFiles()` to recreate directory tree using `RelPath`
- Remove exif/no-exif directory split (breaking change)

**Impact:** Files maintain their original folder structure in destination instead of being split into exif/no-exif folders.

#### Phase 2: Add `--one-dir` Flag
**Status:** ✅ Complete  
**Priority:** High  
**Depends on:** Phase 1

Flatten all files into destination root without subdirectories.

**Changes:**
- ✅ Add `--one-dir` boolean flag
- ✅ Pass flag through `runDedup()` to `copyFiles()`
- ✅ Conditional logic: if enabled, use `filepath.Base()` instead of `RelPath`

**Usage:** `mediadupes -s ./photos -d output --one-dir`

#### Phase 3: Add `--in-place` Mode
**Status:** Planned  
**Priority:** High  
**Depends on:** Phase 1

Delete duplicates in source directory instead of copying to destination.

**Changes:**
- Add `--in-place` boolean flag
- Add `--force` flag for non-interactive confirmation
- Create `deleteFiles()` function to remove duplicates from source
- Add safety confirmation prompt (unless `--force` used)
- Modify `runDedup()` to skip copy and call delete instead
- Validate that `dest` is not provided with `--in-place`

**Safety:**
- Requires explicit confirmation or `--force` flag
- Shows warning about destructive operation
- Displays count of files to be deleted before proceeding

**Usage:** `mediadupes -s ./photos --in-place --force`

## Future Improvements

### High Priority

#### 4. Verbose Mode with Real-time File Display
**Status:** ✅ Complete  
**Priority:** High

Add optional verbose mode showing currently processing files in real-time.

**Changes:**
- ✅ Add `--verbose` or `-V` boolean flag (disabled by default)
- ✅ Create lipgloss box component below progress bar
- ✅ Box size adapts to terminal width (proportional and fixed height)
- ✅ Update `progressMsg` struct to include current file path
- ✅ Workers send current file path when processing
- ✅ Display shows: "Processing: /path/to/current/file.jpg"
- ✅ Auto-truncate paths to fit box width
- ✅ Use `tea.WindowSizeMsg` to detect terminal dimensions
- ✅ Clear display when operation completes

**Usage:** `mediadupes -s ./photos -d output --verbose`

### Medium Priority

#### 1. Delete Duplicates Mode
Delete duplicates in-place instead of copying to new directory.
- Flag: `--delete`
- Safety: Require confirmation or `--force` flag
- Benefit: Clean up without needing extra disk space

#### 2. Size Filtering
Filter files by size to skip thumbnails or focus on large files.
- Flags: `--min-size`, `--max-size`
- Example: `--min-size=1MB` to skip small thumbnails
- Benefit: Faster processing, more relevant results

#### 3. Hash-based Deduplication
True content deduplication instead of filename-based.
- Flag: `--hash` (use SHA256 or similar)
- Current: Deduplicates by filename only
- Benefit: More accurate duplicate detection

#### 4. Hard Links
Create hard links instead of copies to save space.
- Flag: `--link-hard`
- Benefit: Save space while keeping all file references
- Note: Only works within same filesystem

### Medium Priority

#### 5. Soft Links
Create symbolic links instead of copies.
- Flag: `--link-soft`
- Benefit: More flexible than hard links
- Note: Links can break if source moves

#### 6. Partial Matching
Compare only first N bytes for very large files.
- Flag: `--quick` or `--partial-bytes=N`
- Benefit: Faster processing for large videos
- Trade-off: Less accurate

#### 7. One File System
Don't cross filesystem/mount point boundaries.
- Flag: `--one-file-system`
- Benefit: Avoid network drives or external volumes

#### 8. ETA in Progress Bar
Show estimated time remaining.
- Requires: Track processing rate
- Benefit: Better user experience

### Low Priority

#### 9. Dry-run for Copy Mode
Show what would be copied without actually copying.
- Flag: `--dry-run`
- Different from `--plan`: Shows file list, not just stats
- Benefit: Preview before actual operation

#### 10. Interactive Mode
Prompt user to choose which duplicate to keep.
- Flag: `--interactive`
- Benefit: Manual control over deduplication
- Trade-off: Slower, requires user input

#### 11. JSON Output
Machine-readable output for scripting.
- Flag: `--json`
- Benefit: Integration with other tools
- Output: Stats and file lists in JSON format

#### 12. Resume Support
Resume interrupted operations.
- Requires: State file tracking progress
- Benefit: Handle large operations that might fail
- Trade-off: Added complexity

## Performance Optimizations

### Considered but Not Planned

- **Parallel EXIF reading** - exiftool is already the bottleneck
- **Memory-mapped files** - Not beneficial for this use case
- **Custom EXIF parser** - exiftool is mature and reliable

## Code Quality & Refactoring

### Technical Debt to Address

#### 1. Remove Global Mutable State
**Priority:** High  
**Impact:** Testability, Concurrency

**Current Issues:**
- Global atomic counters (`processed`, `unique`, `copied`, `failedScan`, `failedCopy`)
- Global `progChan` channel
- Makes unit testing impossible
- Can't run multiple operations concurrently
- Hidden dependencies between functions

**Solution:**
- Create `Context` or `State` struct to hold all runtime state
- Pass context through function calls
- Enables proper unit testing and concurrent operations

#### 2. Split `runDedup()` Function
**Priority:** High  
**Impact:** Maintainability, Testability

**Current Issues:**
- 112 lines doing too much: validation, UI setup, counting, walking, workers, deduplication, copying, formatting
- Violates Single Responsibility Principle
- Hard to test individual pieces

**Solution:**
- Extract: `countFiles()`, `scanFiles()`, `processResults()`, `displaySummary()`
- Each function should have one clear purpose
- Easier to test and modify independently

#### 3. Replace Boolean Parameter Hell with Config Struct
**Status:** ✅ Complete (v0.1.3)  
**Priority:** High  
**Impact:** Clarity, Maintainability

**Changes:**
- ✅ Created `Config` struct with all configuration fields
- ✅ Extracted `buildConfig()` function for flag parsing
- ✅ Updated all functions to accept `*Config` parameter
- ✅ Simplified `RunE` command handler for better semantic clarity

**Benefits:**
- Clear separation between flag parsing and business logic
- Main execution (`runDedup`) is visually prominent
- Easier to add new configuration options
- Better testability

#### 4. Separate Concerns in `deduplicate()`
**Priority:** Medium  
**Impact:** Clarity, Testability

**Current Issues:**
- Mixes deduplication logic + statistics collection + counter updates
- `updateCounters` closure is clever but confusing
- Hard to test deduplication logic in isolation

**Solution:**
- Pure deduplication function: `func dedup(files []FileInfo) map[string]*FileInfo`
- Separate stats collection: `func calculateStats(files map[string]*FileInfo) Stats`

#### 5. Refactor `copyFiles()` Concurrency
**Priority:** Medium  
**Impact:** Testability, Control

**Current Issues:**
- Creates goroutines internally
- Caller has no control over execution
- Hard to test

**Solution:**
- Return list of copy operations
- Let caller decide how to execute (sequential, parallel, worker pool)
- Or accept a worker pool as parameter

#### 6. Consistent Error Handling
**Priority:** Medium  
**Impact:** Reliability, Debugging

**Current Issues:**
- Some errors sent to `progChan`
- Some errors ignored with `_`
- Some errors returned
- Inconsistent strategy makes debugging hard

**Solution:**
- Decide on one strategy: return errors OR send to channel
- Document error handling policy
- Consider structured logging

#### 7. Extract Magic Numbers to Constants
**Priority:** Low  
**Impact:** Clarity

**Current Issues:**
- `chan string, 1000` - why 1000?
- `chan FileInfo, 100` - why 100?
- No explanation for buffer sizes

**Solution:**
```go
const (
    pathChannelBuffer   = 1000  // Balance memory vs blocking
    resultChannelBuffer = 100   // Smaller since processing is slower
)
```

#### 8. Simplify `walkFiles()` Logic
**Priority:** Low  
**Impact:** Maintainability

**Current Issues:**
- Duplicate filtering logic in recursive/non-recursive paths
- Hard to maintain consistency

**Solution:**
- Extract common filtering logic
- Single source of truth for exclusion rules

#### 9. Clarify `shouldExclude()` Behavior
**Priority:** Low  
**Impact:** Clarity

**Current Issues:**
- Does pattern matching AND substring matching
- Unclear which takes precedence
- Confusing for users and maintainers

**Solution:**
- Split into `matchesPattern()` and `containsSubstring()`
- Document precedence rules
- Or simplify to one matching strategy

#### 10. Optimize `getCreationDate()` Format Detection
**Priority:** Low  
**Impact:** Performance

**Current Issues:**
- Tries 4 field names × 3 date formats = 12 attempts per file
- No caching or early exit optimization

**Solution:**
- Cache successful format per file type
- Fail fast on first successful parse
- Consider reducing format variations

## Notes

- Features are prioritized based on usefulness for media deduplication
- Some features (like hash-based dedup) would require significant refactoring
- Performance testing shows 4-6 workers is optimal for most systems
- Code quality improvements should be done incrementally to avoid breaking changes
