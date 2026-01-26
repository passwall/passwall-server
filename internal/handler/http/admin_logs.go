package http

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/passwall/passwall-server/pkg/constants"
)

type AdminLogsHandler struct {
	appLogPath  string
	httpLogPath string
	maxBytes    int64
}

type AdminLogEntryDTO struct {
	Raw        string `json:"raw"`
	Level      string `json:"level"`
	Timestamp  string `json:"timestamp,omitempty"` // RFC3339
	Version    string `json:"version,omitempty"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Function   string `json:"function,omitempty"`
}

type AdminLogListResponse struct {
	Items     []AdminLogEntryDTO `json:"items"`
	Total     int                `json:"total"`
	Filtered  int                `json:"filtered"`
	Truncated bool               `json:"truncated"`
	LogFile   string             `json:"log_file,omitempty"`
}

func NewAdminLogsHandler() *AdminLogsHandler {
	appLogPath := strings.TrimSpace(os.Getenv(constants.LogPathEnv))
	if appLogPath == "" {
		appLogPath = defaultAppLogPath()
	}
	if st, err := os.Stat(appLogPath); err == nil && st.IsDir() {
		appLogPath = filepath.Join(appLogPath, "passwall-server.log")
	}

	httpLogPath := strings.TrimSpace(os.Getenv(constants.HTTPLogPathEnv))
	if httpLogPath == "" {
		httpLogPath = defaultHTTPLogPath()
	}
	if st, err := os.Stat(httpLogPath); err == nil && st.IsDir() {
		httpLogPath = filepath.Join(httpLogPath, "passwall-http.log")
	}

	return &AdminLogsHandler{
		appLogPath:  appLogPath,
		httpLogPath: httpLogPath,
		maxBytes:    200 * 1024 * 1024, // 200MB tail cap (avoid huge memory use)
	}
}

func defaultAppLogPath() string {
	// Default: next to the running executable (independent from cwd)
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		if exeDir != "" && exeDir != "." {
			return filepath.Join(exeDir, "passwall-server.log")
		}
	}

	return "passwall-server.log"
}

func defaultHTTPLogPath() string {
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		exeDir := filepath.Dir(exePath)
		if exeDir != "" && exeDir != "." {
			return filepath.Join(exeDir, "passwall-http.log")
		}
	}
	return "passwall-http.log"
}

var (
	entryHeaderRe = regexp.MustCompile(`^([A-Z]+)\s+(\S+)\s+(\S+)\s+(.*)$`)
	entrySuffixRe = regexp.MustCompile(`(?s)\sfile:([^:\s]+):(\d+)\s+func:(\S+)\s*$`)
	ginStatusRe   = regexp.MustCompile(`^\[GIN\]\s+(\d{3})\s+\|`)
)

func (h *AdminLogsHandler) List(c *gin.Context) {
	kind := strings.TrimSpace(strings.ToLower(c.DefaultQuery("kind", "app")))
	logPath := h.appLogPath
	if kind == "http" {
		logPath = h.httpLogPath
	}

	limit := clampInt(queryInt(c, "limit", 200), 1, 1000)
	offset := clampInt(queryInt(c, "offset", 0), 0, 10_000_000)
	q := strings.TrimSpace(c.Query("q"))
	level := strings.TrimSpace(strings.ToUpper(c.Query("level")))
	status := 0
	if statusRaw := strings.TrimSpace(c.Query("status")); statusRaw != "" {
		n, err := strconv.Atoi(statusRaw)
		if err != nil || n < 100 || n > 599 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status (100-599 expected)"})
			return
		}
		status = n
	}
	order := strings.TrimSpace(strings.ToLower(c.DefaultQuery("order", "desc")))
	sinceRaw := strings.TrimSpace(c.Query("since"))
	untilRaw := strings.TrimSpace(c.Query("until"))

	var since *time.Time
	if sinceRaw != "" {
		t, err := time.Parse(time.RFC3339, sinceRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since (RFC3339 expected)"})
			return
		}
		since = &t
	}
	var until *time.Time
	if untilRaw != "" {
		t, err := time.Parse(time.RFC3339, untilRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid until (RFC3339 expected)"})
			return
		}
		until = &t
	}

	entries, truncated, err := readAndParseLog(logPath, h.maxBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If the file doesn't exist yet (e.g. no HTTP requests logged yet),
			// return an empty result instead of failing the whole UI.
			c.JSON(http.StatusOK, AdminLogListResponse{
				Items:     []AdminLogEntryDTO{},
				Total:     0,
				Filtered:  0,
				Truncated: false,
				LogFile:   filepath.Base(logPath),
			})
			return
		}
		if errors.Is(err, os.ErrPermission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied reading log file"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read log file"})
		return
	}

	total := len(entries)

	// Apply filters
	filtered := make([]AdminLogEntryDTO, 0, len(entries))
	for _, e := range entries {
		if status != 0 && e.StatusCode != status {
			continue
		}
		if level != "" && strings.ToUpper(e.Level) != level {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Raw), strings.ToLower(q)) {
			continue
		}
		if since != nil || until != nil {
			if e.Timestamp == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, e.Timestamp)
			if err != nil {
				continue
			}
			if since != nil && t.Before(*since) {
				continue
			}
			if until != nil && t.After(*until) {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	if order == "desc" {
		reverseEntries(filtered)
	}

	paged := paginate(filtered, offset, limit)

	c.JSON(http.StatusOK, AdminLogListResponse{
		Items:     paged,
		Total:     total,
		Filtered:  len(filtered),
		Truncated: truncated,
		LogFile:   filepath.Base(logPath),
	})
}

func (h *AdminLogsHandler) Download(c *gin.Context) {
	kind := strings.TrimSpace(strings.ToLower(c.DefaultQuery("kind", "app")))
	logPath := h.appLogPath
	if kind == "http" {
		logPath = h.httpLogPath
	}

	downloadName := strings.TrimSpace(c.Query("filename"))
	if downloadName == "" {
		downloadName = filepath.Base(logPath)
		if downloadName == "." || downloadName == string(filepath.Separator) || downloadName == "" {
			downloadName = filepath.Base(logPath)
			if downloadName == "" {
				downloadName = "passwall-server.log"
			}
		}
	}
	// basic sanitization to avoid header/path injection
	downloadName = filepath.Base(downloadName)

	f, err := os.Open(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "log file not found"})
			return
		}
		if errors.Is(err, os.ErrPermission) {
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied reading log file"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open log file"})
		return
	}
	defer f.Close()

	st, _ := f.Stat()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	if st != nil {
		c.Header("Content-Length", fmt.Sprintf("%d", st.Size()))
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, f)
}

func (h *AdminLogsHandler) DownloadBundle(c *gin.Context) {
	ts := time.Now().UTC().Format("20060102-150405")
	downloadName := fmt.Sprintf("passwall-logs-%s.zip", ts)

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	c.Status(http.StatusOK)

	zw := zip.NewWriter(c.Writer)
	defer func() { _ = zw.Close() }()

	add := func(entryName, path string) {
		w, err := zw.Create(entryName)
		if err != nil {
			return
		}
		f, err := os.Open(path)
		if err != nil {
			// Include a readable placeholder so the bundle is still useful.
			_, _ = w.Write([]byte(fmt.Sprintf("log file not found: %s\n", path)))
			return
		}
		defer f.Close()
		_, _ = io.Copy(w, f)
	}

	add(filepath.Base(h.appLogPath), h.appLogPath)
	add(filepath.Base(h.httpLogPath), h.httpLogPath)
}

func reverseEntries(items []AdminLogEntryDTO) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func paginate(items []AdminLogEntryDTO, offset, limit int) []AdminLogEntryDTO {
	if offset >= len(items) {
		return []AdminLogEntryDTO{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func queryInt(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func readAndParseLog(path string, maxBytes int64) ([]AdminLogEntryDTO, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	var truncated bool
	if st, err := f.Stat(); err == nil {
		if st.Size() > maxBytes {
			truncated = true
			if _, err := f.Seek(st.Size()-maxBytes, 0); err == nil {
				// skip partial first line in tail window
				reader := bufio.NewReader(f)
				_, _ = reader.ReadString('\n')
				entries, err := parseFromReader(reader)
				return entries, truncated, err
			}
		}
	}

	reader := bufio.NewReader(f)
	entries, err := parseFromReader(reader)
	return entries, truncated, err
}

func parseFromReader(reader *bufio.Reader) ([]AdminLogEntryDTO, error) {
	scanner := bufio.NewScanner(reader)
	// Increase buffer for long log entries
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var groups [][]string
	var cur []string
	for scanner.Scan() {
		line := scanner.Text()
		if isEntryStart(line) {
			if len(cur) > 0 {
				groups = append(groups, cur)
			}
			cur = []string{line}
			continue
		}
		if len(cur) == 0 {
			// Orphan continuation line (tail cut) - ignore
			continue
		}
		cur = append(cur, line)
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	out := make([]AdminLogEntryDTO, 0, len(groups))
	for _, g := range groups {
		out = append(out, parseGroup(g))
	}
	return out, nil
}

func isEntryStart(line string) bool {
	return entryHeaderRe.MatchString(line)
}

func parseGroup(lines []string) AdminLogEntryDTO {
	raw := strings.Join(lines, "\n")
	first := ""
	if len(lines) > 0 {
		first = lines[0]
	}
	m := entryHeaderRe.FindStringSubmatch(first)
	if m == nil {
		return AdminLogEntryDTO{
			Raw:     raw,
			Level:   "RAW",
			Message: raw,
		}
	}

	level := m[1]
	tsRaw := m[2]
	version := m[3]
	firstMsg := m[4]

	var ts string
	if t, err := time.Parse(time.RFC3339, tsRaw); err == nil {
		ts = t.Format(time.RFC3339)
	}

	// Extract suffix (file/line/func) from the whole raw (multi-line aware)
	file := ""
	function := ""
	lineNum := 0
	body := raw
	if sm := entrySuffixRe.FindStringSubmatchIndex(body); sm != nil {
		file = body[sm[2]:sm[3]]
		lnRaw := body[sm[4]:sm[5]]
		function = body[sm[6]:sm[7]]
		if n, err := strconv.Atoi(lnRaw); err == nil {
			lineNum = n
		}
		body = strings.TrimRight(body[:sm[0]], " \n\t")
	}

	// Build message: strip "LEVEL ts version " prefix from first line, keep other lines
	bodyLines := strings.Split(body, "\n")
	msgLines := make([]string, 0, len(bodyLines))
	msgLines = append(msgLines, strings.TrimRight(firstMsg, " "))
	if len(bodyLines) > 1 {
		msgLines = append(msgLines, bodyLines[1:]...)
	}
	message := strings.TrimRight(strings.Join(msgLines, "\n"), " \n\t")

	statusCode := 0
	// Parse HTTP status code from Gin log line: "[GIN] 200 | ..."
	if firstLine := strings.SplitN(message, "\n", 2)[0]; strings.HasPrefix(firstLine, "[GIN]") {
		if sm := ginStatusRe.FindStringSubmatch(firstLine); sm != nil {
			if n, err := strconv.Atoi(sm[1]); err == nil {
				statusCode = n
			}
		}
	}

	return AdminLogEntryDTO{
		Raw:        raw,
		Level:      level,
		Timestamp:  ts,
		Version:    version,
		Message:    message,
		StatusCode: statusCode,
		File:       file,
		Line:       lineNum,
		Function:   function,
	}
}
