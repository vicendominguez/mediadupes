package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/barasher/go-exiftool"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type FileInfo struct {
	Path         string
	RelPath      string
	Size         int64
	HasMeta      bool
	BaseName     string
	CreationDate time.Time
	IsImage      bool
}

type progressMsg struct {
	stage       string
	current     int64
	total       int64
	done        bool
	summary     string
	currentFile string
	errorFile   string
	errorMsg    string
}

type model struct {
	progress    progress.Model
	stage       string
	current     int64
	total       int64
	done        bool
	verbose     bool
	currentFile string
	width       int
	errors      []string
	errorCount  int
	summary  string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case progressMsg:
		m.stage = msg.stage
		m.current = msg.current
		m.total = msg.total
		m.done = msg.done
		m.summary = msg.summary
		m.currentFile = msg.currentFile
		if msg.errorFile != "" {
			errorEntry := fmt.Sprintf("%s: %s", msg.errorFile, msg.errorMsg)
			m.errors = append(m.errors, errorEntry)
			if len(m.errors) > 10 {
				m.errors = m.errors[1:]
			}
			m.errorCount++
		}
		if m.done {
			return m, tea.Quit
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "v" {
			m.verbose = !m.verbose
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.done {
		return m.summary + "\n"
	}
	percent := 0.0
	if m.total > 0 {
		percent = float64(m.current) / float64(m.total)
	}
	
	view := fmt.Sprintf("\n%s\n%s %d/%d\n", m.stage, m.progress.ViewAs(percent), m.current, m.total)
	
	if m.verbose && m.currentFile != "" {
		boxWidth := m.width - 4
		if boxWidth < 20 {
			boxWidth = 20
		}
		
		displayPath := m.currentFile
		if len(displayPath) > boxWidth-2 {
			displayPath = "..." + displayPath[len(displayPath)-(boxWidth-5):]
		}
		
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1).
			Width(boxWidth)
		
		view += "\n" + boxStyle.Render(fmt.Sprintf("Processing: %s", displayPath)) + "\n"
	}
	
	if len(m.errors) > 0 {
		boxWidth := m.width - 4
		if boxWidth < 30 {
			boxWidth = 30
		}
		
		errorStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(0, 1).
			Width(boxWidth)
		
		errorLines := fmt.Sprintf("⚠️  Errors (%d total) - Last %d:", m.errorCount, len(m.errors))
		for _, err := range m.errors {
			if len(err) > boxWidth-4 {
				err = "..." + err[len(err)-(boxWidth-7):]
			}
			errorLines += "\n" + err
		}
		
		view += "\n" + errorStyle.Render(errorLines) + "\n"
	}
	
	return view
}

var (
	processed   atomic.Int64
	unique      atomic.Int64
	copied      atomic.Int64
	failedScan  atomic.Int64
	failedCopy  atomic.Int64
	progChan    chan progressMsg
	version     = "0.1.0"
)

func getCreationDate(et *exiftool.Exiftool, filePath string) (time.Time, bool) {
	metas := et.ExtractMetadata(filePath)
	if len(metas) == 0 {
		return time.Time{}, false
	}
	meta := metas[0]
	if meta.Err != nil {
		return time.Time{}, false
	}

	fields := []string{"DateTimeOriginal", "CreateDate", "CreationDate", "MediaCreateDate"}
	for _, field := range fields {
		if val, ok := meta.Fields[field]; ok {
			if dateStr, ok := val.(string); ok {
				formats := []string{
					"2006:01:02 15:04:05",
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
				}
				for _, format := range formats {
					if t, err := time.Parse(format, dateStr); err == nil {
						return t, true
					}
				}
			}
		}
	}
	return time.Time{}, false
}

func shouldExclude(path string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func walkFiles(source string, pathChan chan<- string, validExts map[string]bool, recursive bool, excludePatterns []string) {
	defer close(pathChan)

	if !recursive {
		entries, err := os.ReadDir(source)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(source, entry.Name())
			if shouldExclude(path, excludePatterns) {
				continue
			}
			ext := strings.ToLower(filepath.Ext(path))
			if validExts[ext] {
				pathChan <- path
			}
		}
		return
	}

	filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldExclude(path, excludePatterns) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldExclude(path, excludePatterns) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if validExts[ext] {
			pathChan <- path
		}
		return nil
	})
}

func worker(et *exiftool.Exiftool, source string, pathChan <-chan string, resultChan chan<- FileInfo, total *atomic.Int64, checkMeta bool, stage string, imageExts map[string]bool) {
	for path := range pathChan {
		stat, err := os.Stat(path)
		if err != nil {
			failedScan.Add(1)
			progChan <- progressMsg{errorFile: path, errorMsg: err.Error()}
			continue
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			relPath = filepath.Base(path)
		}

		ext := filepath.Ext(path)
		baseName := strings.TrimSuffix(filepath.Base(path), ext)
		
		var creationDate time.Time
		var hasMeta bool
		if checkMeta {
			creationDate, hasMeta = getCreationDate(et, path)
		}

		info := FileInfo{
			Path:         path,
			RelPath:      relPath,
			Size:         stat.Size(),
			BaseName:     baseName,
			HasMeta:      hasMeta,
			CreationDate: creationDate,
			IsImage:      imageExts[strings.ToLower(ext)],
		}

		resultChan <- info
		current := processed.Add(1)
		progChan <- progressMsg{stage: stage, current: current, total: total.Load(), currentFile: path}
	}
}

type Stats struct {
	TotalSize    int64
	UniqueSize   int64
	TotalImages  int
	TotalVideos  int
	UniqueImages int
	UniqueVideos int
	WithMeta     int
	WithoutMeta  int
}

func deduplicate(resultChan <-chan FileInfo, enableDedup bool) (map[string]*FileInfo, Stats) {
	dedup := make(map[string]*FileInfo)
	var mu sync.Mutex
	stats := Stats{}

	updateCounters := func(info *FileInfo, add bool) {
		delta := 1
		if !add {
			delta = -1
		}
		if info.IsImage {
			stats.UniqueImages += delta
		} else {
			stats.UniqueVideos += delta
		}
	}

	for info := range resultChan {
		mu.Lock()
		stats.TotalSize += info.Size
		
		if info.IsImage {
			stats.TotalImages++
		} else {
			stats.TotalVideos++
		}
		
		if info.HasMeta {
			stats.WithMeta++
		} else {
			stats.WithoutMeta++
		}
		
		if !enableDedup {
			key := info.Path
			dedup[key] = &info
			stats.UniqueSize += info.Size
			updateCounters(&info, true)
			mu.Unlock()
			continue
		}
		
		existing, exists := dedup[info.BaseName]

		if !exists {
			dedup[info.BaseName] = &info
			stats.UniqueSize += info.Size
			updateCounters(&info, true)
		} else {
			shouldReplace := (info.HasMeta && !existing.HasMeta) ||
				(info.HasMeta == existing.HasMeta && info.Size > existing.Size)
			
			if shouldReplace {
				stats.UniqueSize = stats.UniqueSize - existing.Size + info.Size
				updateCounters(existing, false)
				updateCounters(&info, true)
				dedup[info.BaseName] = &info
			}
		}
		mu.Unlock()
	}

	return dedup, stats
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func copyFiles(files map[string]*FileInfo, dest string, parallel int, checkMeta, oneDir bool) {
	os.MkdirAll(dest, 0755)

	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	total := int64(len(files))

	for _, info := range files {
		wg.Add(1)
		go func(fi *FileInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			src, err := os.Open(fi.Path)
			if err != nil {
				failedCopy.Add(1)
				progChan <- progressMsg{errorFile: fi.Path, errorMsg: fmt.Sprintf("open: %v", err)}
				return
			}
			defer src.Close()

			var destPath string
			if oneDir {
				destPath = filepath.Join(dest, filepath.Base(fi.Path))
			} else {
				destPath = filepath.Join(dest, fi.RelPath)
			}
			
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				failedCopy.Add(1)
				progChan <- progressMsg{errorFile: fi.Path, errorMsg: fmt.Sprintf("mkdir: %v", err)}
				return
			}

			dst, err := os.Create(destPath)
			if err != nil {
				failedCopy.Add(1)
				progChan <- progressMsg{errorFile: fi.Path, errorMsg: fmt.Sprintf("create: %v", err)}
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err == nil {
				if checkMeta && fi.HasMeta && !fi.CreationDate.IsZero() {
					os.Chtimes(destPath, fi.CreationDate, fi.CreationDate)
				}
				current := copied.Add(1)
				progChan <- progressMsg{stage: "💾 Copying files...", current: current, total: total, currentFile: fi.Path}
			} else {
				failedCopy.Add(1)
				progChan <- progressMsg{errorFile: fi.Path, errorMsg: fmt.Sprintf("copy: %v", err)}
			}
		}(info)
	}

	wg.Wait()
}

func runDedup(source, dest string, workers, copyParallel int, validExts map[string]bool, imageExts map[string]bool, recursive, checkMeta, enableDedup, planOnly, oneDir, verbose bool, excludePatterns []string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("source directory '%s' does not exist", source)
	}

	progChan = make(chan progressMsg, 10)
	prog := progress.New(progress.WithDefaultGradient())
	m := model{progress: prog, verbose: verbose, width: 80}

	p := tea.NewProgram(m)
	go p.Run()
	
	time.Sleep(50 * time.Millisecond)
	progChan <- progressMsg{stage: "🔍 Counting files...", current: 0, total: 1}

	var totalFiles atomic.Int64
	filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && validExts[strings.ToLower(filepath.Ext(path))] {
			totalFiles.Add(1)
		}
		return nil
	})

	go func() {
		for msg := range progChan {
			p.Send(msg)
		}
	}()

	pathChan := make(chan string, 1000)
	resultChan := make(chan FileInfo, 100)

	go walkFiles(source, pathChan, validExts, recursive, excludePatterns)

	var wg sync.WaitGroup
	startTime := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			et, err := exiftool.NewExiftool()
			if err != nil {
				progChan <- progressMsg{errorFile: "exiftool", errorMsg: fmt.Sprintf("worker init failed: %v", err)}
				return
			}
			defer et.Close()
			
			worker(et, source, pathChan, resultChan, &totalFiles, checkMeta, "📁 Scanning files...", imageExts)
		}()
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	dedup, stats := deduplicate(resultChan, enableDedup)
	unique.Store(int64(len(dedup)))
	elapsed := time.Since(startTime)

	if planOnly {
		savings := stats.TotalSize - stats.UniqueSize
		savingsPercent := 0.0
		if stats.TotalSize > 0 {
			savingsPercent = float64(savings) / float64(stats.TotalSize) * 100
		}
		
		totalImageSize := int64(0)
		totalVideoSize := int64(0)
		for _, info := range dedup {
			if info.IsImage {
				totalImageSize += info.Size
			} else {
				totalVideoSize += info.Size
			}
		}
		
		// Styles
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
		savingsStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		
		summary := fmt.Sprintf("\n%s\n\n%s\n  %s %s\n  %s %s\n  %s %d\n  %s %s\n\n%s\n  %s %d files (%s)\n  %s %d files (%s)\n\n%s\n  %s %d files\n  %s %d files\n\n%s\n  %s %d\n  %s %.1fs",
			titleStyle.Render("Summary"),
			labelStyle.Render("Files:"),
			labelStyle.Render("  Total:"), valueStyle.Render(fmt.Sprintf("%d (%s)", processed.Load(), formatBytes(stats.TotalSize))),
			labelStyle.Render("  Unique:"), valueStyle.Render(fmt.Sprintf("%d (%s)", unique.Load(), formatBytes(stats.UniqueSize))),
			labelStyle.Render("  Duplicates:"), processed.Load()-unique.Load(),
			labelStyle.Render("  Savings:"), savingsStyle.Render(fmt.Sprintf("%s (%.1f%%)", formatBytes(savings), savingsPercent)),
			labelStyle.Render("By Type:"),
			labelStyle.Render("  Images:"), stats.TotalImages, formatBytes(totalImageSize),
			labelStyle.Render("  Videos:"), stats.TotalVideos, formatBytes(totalVideoSize),
			labelStyle.Render("Metadata:"),
			labelStyle.Render("  With EXIF:"), stats.WithMeta,
			labelStyle.Render("  Without EXIF:"), stats.WithoutMeta,
			labelStyle.Render("Performance:"),
			labelStyle.Render("  Workers:"), workers,
			labelStyle.Render("  Time:"), elapsed.Seconds())
		
		progChan <- progressMsg{done: true, summary: summary}
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	copyFiles(dedup, dest, copyParallel, checkMeta, oneDir)

	failedScanCount := failedScan.Load()
	failedCopyCount := failedCopy.Load()
	
	summary := fmt.Sprintf("\nComplete!\n   Processed: %d files\n   Unique: %d files\n   Copied: %d files\n   Failed: %d files (%d scan, %d copy)", 
		processed.Load(), unique.Load(), copied.Load(), failedScanCount+failedCopyCount, failedScanCount, failedCopyCount)
	
	progChan <- progressMsg{done: true, summary: summary}
	time.Sleep(200 * time.Millisecond)

	return nil
}

func main() {
	var source, dest string
	var workers, copyParallel int
	var imageExts, videoExts, excludeStr string
	var noRecursive, noMeta, noDedup, planOnly, showVersion, oneDir, verbose bool

	rootCmd := &cobra.Command{
		Use:   "mediadupes",
		Short: "Deduplicate photos and videos based on metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Printf("mediadupes version %s\n", version)
				return nil
			}
			
			if !cmd.Flags().Changed("source") && !cmd.Flags().Changed("dest") && !planOnly {
				return cmd.Help()
			}
			
			validExts := make(map[string]bool)
			imageExtsMap := make(map[string]bool)
			for _, ext := range strings.Split(imageExts, ",") {
				if ext = strings.TrimSpace(ext); ext != "" {
					validExts[strings.ToLower(ext)] = true
					imageExtsMap[strings.ToLower(ext)] = true
				}
			}
			for _, ext := range strings.Split(videoExts, ",") {
				if ext = strings.TrimSpace(ext); ext != "" {
					validExts[strings.ToLower(ext)] = true
				}
			}
			
			var excludePatterns []string
			if cmd.Flags().Changed("exclude") {
				// Usuario especificó --exclude, usar solo esos patrones
				for _, pattern := range strings.Split(excludeStr, ",") {
					if pattern = strings.TrimSpace(pattern); pattern != "" {
						excludePatterns = append(excludePatterns, pattern)
					}
				}
			} else {
				// Por defecto: excluir directorios con punto
				excludePatterns = []string{".*"}
			}
			
			return runDedup(source, dest, workers, copyParallel, validExts, imageExtsMap, !noRecursive, !noMeta, !noDedup, planOnly, oneDir, verbose, excludePatterns)
		},
	}

	rootCmd.Flags().StringVarP(&source, "source", "s", ".", "Source directory (default: current directory)")
	rootCmd.Flags().StringVarP(&dest, "dest", "d", "MEDIADUPES", "Destination directory (default: MEDIADUPES)")
	rootCmd.Flags().IntVarP(&workers, "workers", "w", runtime.NumCPU(), "Number of workers (default: CPU count)")
	rootCmd.Flags().IntVarP(&copyParallel, "copy-parallel", "c", 4, "Parallel copy operations (default: 4)")
	rootCmd.Flags().StringVar(&imageExts, "image-exts", ".jpg,.jpeg,.png,.heic,.heif", "Image extensions (default: .jpg,.jpeg,.png,.heic,.heif)")
	rootCmd.Flags().StringVar(&videoExts, "video-exts", ".mp4,.mov,.avi,.mkv,.m4v", "Video extensions (default: .mp4,.mov,.avi,.mkv,.m4v)")
	rootCmd.Flags().StringVar(&excludeStr, "exclude", "", "Exclude patterns (default: directories starting with '.', override with comma-separated patterns)")
	rootCmd.Flags().BoolVar(&noRecursive, "no-recursive", false, "Disable recursive directory scanning (default: false, recursive enabled)")
	rootCmd.Flags().BoolVar(&noMeta, "no-meta", false, "Disable metadata/EXIF checking (default: false, metadata enabled)")
	rootCmd.Flags().BoolVar(&noDedup, "no-dedup", false, "Disable deduplication (default: false, dedup enabled)")
	rootCmd.Flags().BoolVar(&planOnly, "plan", false, "Show space savings estimate without copying (default: false)")
	rootCmd.Flags().BoolVar(&oneDir, "one-dir", false, "Copy all files to dest root without subdirectories (default: false)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "V", false, "Show currently processing files in real-time (default: false)")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
