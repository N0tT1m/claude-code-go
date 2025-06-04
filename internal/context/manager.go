// Package: internal/context/manager.go
package context

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ContextManager struct {
	projectRoot string
	maxTokens   int
	cache       map[string]*FileContext
	lastRefresh time.Time
	refreshTTL  time.Duration
}

type FileContext struct {
	Path         string
	Content      string
	Size         int
	LastModified time.Time
	Hash         string
	Language     string
	TokenCount   int
}

type ProjectContext struct {
	Files        []FileContext
	Structure    string
	Dependencies []string
	GitInfo      GitContext
	TotalTokens  int
}

type GitContext struct {
	Branch        string
	CommitHash    string
	Status        string
	RecentCommits []string
}

func NewContextManager(projectRoot string, maxTokens int) *ContextManager {
	return &ContextManager{
		projectRoot: projectRoot,
		maxTokens:   maxTokens,
		cache:       make(map[string]*FileContext),
		refreshTTL:  5 * time.Minute,
	}
}

func (cm *ContextManager) GetProjectContext() (*ProjectContext, error) {
	if time.Since(cm.lastRefresh) > cm.refreshTTL {
		cm.refreshCache()
	}

	files, err := cm.getRelevantFiles()
	if err != nil {
		return nil, err
	}

	structure, err := cm.generateProjectStructure()
	if err != nil {
		return nil, err
	}

	gitInfo, err := cm.getGitContext()
	if err != nil {
		// Git context is optional
		gitInfo = GitContext{}
	}

	deps, err := cm.getDependencies()
	if err != nil {
		deps = []string{} // Dependencies are optional
	}

	totalTokens := cm.calculateTotalTokens(files)

	return &ProjectContext{
		Files:        files,
		Structure:    structure,
		Dependencies: deps,
		GitInfo:      gitInfo,
		TotalTokens:  totalTokens,
	}, nil
}

func (cm *ContextManager) refreshCache() {
	cm.cache = make(map[string]*FileContext)
	cm.lastRefresh = time.Now()
}

func (cm *ContextManager) getRelevantFiles() ([]FileContext, error) {
	var files []FileContext
	tokenCount := 0

	err := filepath.WalkDir(cm.projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") && d.Name() != ".env" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common non-source directories
		if d.IsDir() {
			skipDirs := []string{"node_modules", "vendor", "target", "build", "dist", ".git"}
			for _, skip := range skipDirs {
				if d.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Only include source files
		if !cm.isSourceFile(path) {
			return nil
		}

		fileCtx, err := cm.getFileContext(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Respect token limit
		if tokenCount+fileCtx.TokenCount > cm.maxTokens {
			return nil
		}

		files = append(files, *fileCtx)
		tokenCount += fileCtx.TokenCount

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by relevance (recently modified first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})

	return files, nil
}

func (cm *ContextManager) getFileContext(path string) (*FileContext, error) {
	// Check cache first
	if cached, exists := cm.cache[path]; exists {
		stat, err := os.Stat(path)
		if err == nil && stat.ModTime().Equal(cached.LastModified) {
			return cached, nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	hash := fmt.Sprintf("%x", md5.Sum(content))

	fileCtx := &FileContext{
		Path:         path,
		Content:      string(content),
		Size:         len(content),
		LastModified: stat.ModTime(),
		Hash:         hash,
		Language:     cm.detectLanguage(path),
		TokenCount:   cm.estimateTokens(string(content)),
	}

	cm.cache[path] = fileCtx
	return fileCtx, nil
}

func (cm *ContextManager) isSourceFile(path string) bool {
	sourceExts := []string{
		".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".php", ".rb", ".rs", ".swift", ".kt", ".scala", ".clj", ".hs", ".ml",
		".R", ".jl", ".dart", ".lua", ".sh", ".bash", ".zsh", ".fish", ".ps1",
		".sql", ".html", ".css", ".scss", ".sass", ".less", ".vue", ".svelte",
		".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf", ".env",
		".md", ".rst", ".txt", ".dockerfile", "Dockerfile", "Makefile",
	}

	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	for _, sourceExt := range sourceExts {
		if ext == sourceExt || base == strings.ToLower(sourceExt) {
			return true
		}
	}

	return false
}

func (cm *ContextManager) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	langMap := map[string]string{
		".go":        "go",
		".py":        "python",
		".js":        "javascript",
		".ts":        "typescript",
		".jsx":       "javascript",
		".tsx":       "typescript",
		".java":      "java",
		".c":         "c",
		".cpp":       "cpp",
		".h":         "c",
		".hpp":       "cpp",
		".cs":        "csharp",
		".php":       "php",
		".rb":        "ruby",
		".rs":        "rust",
		".swift":     "swift",
		".kt":        "kotlin",
		".scala":     "scala",
		".clj":       "clojure",
		".hs":        "haskell",
		".ml":        "ocaml",
		".r":         "r",
		".jl":        "julia",
		".dart":      "dart",
		".lua":       "lua",
		".sh":        "bash",
		".bash":      "bash",
		".zsh":       "zsh",
		".fish":      "fish",
		".ps1":       "powershell",
		".sql":       "sql",
		".html":      "html",
		".css":       "css",
		".scss":      "scss",
		".sass":      "sass",
		".less":      "less",
		".vue":       "vue",
		".svelte":    "svelte",
		".yaml":      "yaml",
		".yml":       "yaml",
		".json":      "json",
		".toml":      "toml",
		".ini":       "ini",
		".cfg":       "ini",
		".conf":      "ini",
		".env":       "env",
		".md":        "markdown",
		".rst":       "rst",
		".txt":       "text",
		"dockerfile": "dockerfile",
		"makefile":   "makefile",
	}

	if lang, exists := langMap[ext]; exists {
		return lang
	}

	if lang, exists := langMap[base]; exists {
		return lang
	}

	return "unknown"
}

func (cm *ContextManager) estimateTokens(content string) int {
	// Rough estimation: ~4 characters per token
	return len(content) / 4
}

func (cm *ContextManager) generateProjectStructure() (string, error) {
	var structure strings.Builder

	err := filepath.WalkDir(cm.projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories except .env
		if strings.HasPrefix(d.Name(), ".") && d.Name() != ".env" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common non-source directories
		if d.IsDir() {
			skipDirs := []string{"node_modules", "vendor", "target", "build", "dist", ".git"}
			for _, skip := range skipDirs {
				if d.Name() == skip {
					return filepath.SkipDir
				}
			}
		}

		relPath, err := filepath.Rel(cm.projectRoot, path)
		if err != nil {
			relPath = path
		}

		depth := strings.Count(relPath, string(filepath.Separator))
		indent := strings.Repeat("  ", depth)

		if d.IsDir() {
			structure.WriteString(fmt.Sprintf("%s%s/\n", indent, d.Name()))
		} else {
			structure.WriteString(fmt.Sprintf("%s%s\n", indent, d.Name()))
		}

		return nil
	})

	return structure.String(), err
}

func (cm *ContextManager) getGitContext() (GitContext, error) {
	// This would execute git commands to get context
	// Simplified implementation
	return GitContext{
		Branch:        "main",
		CommitHash:    "abc123",
		Status:        "clean",
		RecentCommits: []string{"Initial commit"},
	}, nil
}

func (cm *ContextManager) getDependencies() ([]string, error) {
	var deps []string

	// Check for Go dependencies
	if goMod := filepath.Join(cm.projectRoot, "go.mod"); cm.fileExists(goMod) {
		deps = append(deps, "Go project (go.mod found)")
	}

	// Check for Node.js dependencies
	if packageJSON := filepath.Join(cm.projectRoot, "package.json"); cm.fileExists(packageJSON) {
		deps = append(deps, "Node.js project (package.json found)")
	}

	// Check for Python dependencies
	if requirements := filepath.Join(cm.projectRoot, "requirements.txt"); cm.fileExists(requirements) {
		deps = append(deps, "Python project (requirements.txt found)")
	}

	if pyproject := filepath.Join(cm.projectRoot, "pyproject.toml"); cm.fileExists(pyproject) {
		deps = append(deps, "Python project (pyproject.toml found)")
	}

	return deps, nil
}

func (cm *ContextManager) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (cm *ContextManager) calculateTotalTokens(files []FileContext) int {
	total := 0
	for _, file := range files {
		total += file.TokenCount
	}
	return total
}
