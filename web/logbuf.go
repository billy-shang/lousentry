package web

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kataras/golog"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// LogEntry 控制台展示的一条运行日志。
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// LogBuffer 保存最近的日志，同时继续输出到 stdout。
type LogBuffer struct {
	mu       sync.Mutex
	items    []LogEntry
	max      int
	leftover []byte
}

func NewLogBuffer(max int) *LogBuffer {
	if max <= 0 {
		max = 2000
	}
	return &LogBuffer{max: max}
}

func (b *LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.leftover = append(b.leftover, p...)
	for {
		i := bytes.IndexByte(b.leftover, '\n')
		if i < 0 {
			break
		}
		text := strings.TrimSpace(ansiRe.ReplaceAllString(string(b.leftover[:i]), ""))
		b.leftover = append([]byte{}, b.leftover[i+1:]...)
		if text == "" {
			continue
		}
		b.items = append(b.items, parseLogLine(text))
		if len(b.items) > b.max {
			b.items = b.items[len(b.items)-b.max:]
		}
	}
	return len(p), nil
}

func (b *LogBuffer) List(level string, page, size int) ([]LogEntry, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 100
	}
	var filtered []LogEntry
	level = strings.ToLower(strings.TrimSpace(level))
	for i := len(b.items) - 1; i >= 0; i-- {
		item := b.items[i]
		if level != "" && !strings.EqualFold(item.Level, level) {
			continue
		}
		filtered = append(filtered, item)
	}
	total := len(filtered)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func (b *LogBuffer) Attach() {
	golog.AddOutput(b)
}

func parseLogLine(line string) LogEntry {
	entry := LogEntry{
		Time:    time.Now().Format("2006-01-02 15:04:05"),
		Level:   "info",
		Message: line,
	}
	if strings.HasPrefix(line, "[") {
		if i := strings.Index(line, "]"); i > 1 {
			raw := strings.ToLower(strings.TrimSpace(line[1:i]))
			switch raw {
			case "erro", "error", "fatal":
				entry.Level = "error"
			case "warn", "warning":
				entry.Level = "warn"
			case "dbug", "debug":
				entry.Level = "debug"
			default:
				entry.Level = "info"
			}
			rest := strings.TrimSpace(line[i+1:])
			if len(rest) >= 19 {
				if _, err := time.Parse("2006/01/02 15:04:05", rest[:19]); err == nil {
					entry.Time = strings.Replace(rest[:19], "/", "-", 2)
					entry.Message = strings.TrimSpace(rest[19:])
					return entry
				}
			}
			entry.Message = rest
		}
	}
	return entry
}
