# mediadupes

Fast, parallel media deduplicator with EXIF metadata support.

## Features

- Deduplicates photos and videos by filename
- Prioritizes files with EXIF metadata
- Preserves original directory structure
- Parallel processing for speed
- Recursive directory scanning

## Install

### Homebrew (macOS)

```bash
brew tap vicendominguez/tap
brew install mediadupes
```

### From Release (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/YOUR_USERNAME/mediadupes/releases):

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/YOUR_USERNAME/mediadupes/releases/latest/download/mediadupes-darwin-arm64.tar.gz | tar xz
sudo mv mediadupes /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/YOUR_USERNAME/mediadupes/releases/latest/download/mediadupes-darwin-amd64.tar.gz | tar xz
sudo mv mediadupes /usr/local/bin/
```

**Linux (amd64):**
```bash
curl -L https://github.com/YOUR_USERNAME/mediadupes/releases/latest/download/mediadupes-linux-amd64.tar.gz | tar xz
sudo mv mediadupes /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/YOUR_USERNAME/mediadupes/releases/latest/download/mediadupes-linux-arm64.tar.gz | tar xz
sudo mv mediadupes /usr/local/bin/
```

**Debian/Ubuntu:**
```bash
curl -LO https://github.com/YOUR_USERNAME/mediadupes/releases/latest/download/mediadupes_VERSION_amd64.deb
sudo dpkg -i mediadupes_VERSION_amd64.deb
```

**Windows:**
Download `mediadupes-windows-amd64.zip` from releases and extract to your PATH.

### From Source

```bash
go install
```

Or build locally:

```bash
mise run build
```

## Usage

Basic usage:

```bash
mediadupes -s /path/to/photos -d output
```

The tool will:
1. Scan the source directory recursively
2. Identify duplicates by filename
3. Keep files with EXIF metadata when duplicates exist
4. Copy unique files to destination, preserving the original directory structure

### Examples

Scan current directory:
```bash
mediadupes -s . -d organized
```

Check space savings without copying:
```bash
mediadupes -s ./photos --plan
```

Example output:
```
Summary

Files:
    Total: 15689 (52.6 GB)
    Unique: 14123 (43.2 GB)
    Duplicates: 1566
    Savings: 9.4 GB (17.8%)

By Type:
    Images: 13898 files (19.4 GB)
    Videos: 1791 files (23.8 GB)

Metadata:
    With EXIF: 15584 files
    Without EXIF: 105 files

Performance:
    Workers: 12
    Time: 19.1s
```

Only top-level directory (no recursion):
```bash
mediadupes -s ./photos -d output --no-recursive
```

Skip metadata checking (faster, no prioritization):
```bash
mediadupes -s ./photos -d output --no-meta
```

Copy all files without deduplication:
```bash
mediadupes -s ./photos -d output --no-dedup
```

Custom file extensions:
```bash
mediadupes -s ./photos -d output --image-exts=".jpg,.png" --video-exts=".mp4"
```

Exclude patterns:
```bash
mediadupes -s ./photos -d output --exclude=".DS_Store,Thumbs.db,*.tmp"
```

Flatten all files to destination root (no subdirectories):
```bash
mediadupes -s ./photos -d output --one-dir
```

Show currently processing files in real-time:
```bash
mediadupes -s ./photos -d output --verbose
```

### Interactive Controls

While the progress bar is running:
- Press **`v`** to toggle verbose mode on/off (show/hide currently processing files)
- Press **`Ctrl+C`** to quit

### Error Handling

The tool automatically tracks and displays errors in real-time:
- **Error box appears** when files fail to scan or copy
- Shows **last 10 errors** with file paths and error messages
- **Total error count** displayed in final summary
- Errors are categorized as **scan errors** (file access issues) or **copy errors** (write failures)

## Options

```
-s, --source          Source directory (default: ".")
-d, --dest            Destination directory (default: "MEDIADUPES")
-w, --workers         Number of workers (default: CPU count)
-c, --copy-parallel   Parallel copy operations (default: 4)
    --plan            Show space savings estimate without copying (default: false)
    --image-exts      Image extensions (default: ".jpg,.jpeg,.png,.heic,.heif")
    --video-exts      Video extensions (default: ".mp4,.mov,.avi,.mkv,.m4v")
    --exclude         Exclude patterns (comma-separated, default: directories starting with ".")
    --no-recursive    Disable recursive scanning (default: false, recursive enabled)
    --no-meta         Disable metadata checking (default: false, metadata enabled)
    --no-dedup        Disable deduplication (default: false, dedup enabled)
    --one-dir         Copy all files to dest root without subdirectories (default: false)
-V, --verbose         Show currently processing files in real-time (default: false)
-v, --version         Show version information
```

**Note:** Directories starting with `.` are automatically excluded unless overridden with `--exclude`.

## Requirements

- Go 1.21+
- [exiftool](https://exiftool.org/) - Install with `brew install exiftool`
