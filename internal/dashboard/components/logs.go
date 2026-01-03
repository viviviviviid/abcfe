package components

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abcfe/abcfe-node/internal/dashboard/styles"
)

// LogViewer는 로그 파일 뷰어
type LogViewer struct {
	logDir      string
	nodeIndex   int
	lines       []LogLine
	maxLines    int
	lastModTime time.Time
}

// LogLine은 파싱된 로그 라인
type LogLine struct {
	Time    string
	Level   string
	Message string
	Raw     string
}

// NewLogViewer는 새 로그 뷰어 생성
func NewLogViewer(logDir string, maxLines int) *LogViewer {
	return &LogViewer{
		logDir:   logDir,
		maxLines: maxLines,
		lines:    make([]LogLine, 0),
	}
}

// SetNode는 보여줄 노드 설정
func (lv *LogViewer) SetNode(nodeIndex int) {
	lv.nodeIndex = nodeIndex
	lv.lines = nil
	lv.lastModTime = time.Time{}
}

// GetLogPath는 현재 노드의 로그 파일 경로 반환
func (lv *LogViewer) GetLogPath() string {
	today := time.Now().Format("2006-01-02")

	// 노드별 로그 경로
	var logPath string
	if lv.nodeIndex == 0 {
		logPath = filepath.Join(lv.logDir, "syslogs", fmt.Sprintf("_%s.log", today))
	} else {
		logPath = filepath.Join(lv.logDir, fmt.Sprintf("syslogs%d", lv.nodeIndex+1), fmt.Sprintf("_%s.log", today))
	}

	return logPath
}

// Refresh는 로그 파일을 다시 읽음
func (lv *LogViewer) Refresh() error {
	logPath := lv.GetLogPath()

	// 파일 존재 확인
	info, err := os.Stat(logPath)
	if err != nil {
		// 파일이 없으면 빈 로그
		lv.lines = []LogLine{{
			Level:   "INFO",
			Message: fmt.Sprintf("로그 파일 없음: %s", logPath),
		}}
		return nil
	}

	// 수정 시간이 같으면 스킵
	if info.ModTime().Equal(lv.lastModTime) {
		return nil
	}
	lv.lastModTime = info.ModTime()

	// 파일 읽기
	file, err := os.Open(logPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var allLines []LogLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parsed := lv.parseLine(line)
		allLines = append(allLines, parsed)
	}

	// 마지막 maxLines만 유지
	if len(allLines) > lv.maxLines {
		allLines = allLines[len(allLines)-lv.maxLines:]
	}

	lv.lines = allLines
	return nil
}

// parseLine은 로그 라인을 파싱
func (lv *LogViewer) parseLine(line string) LogLine {
	// JSON 로그 파싱 시도
	// {"date":"2025-01-03T12:00:00Z","level":"INFO","msg":"info","Info":"message"}

	result := LogLine{Raw: line}

	// 간단한 파싱 (정규식 없이)
	if strings.Contains(line, `"level":"DEBUG"`) {
		result.Level = "DEBUG"
	} else if strings.Contains(line, `"level":"INFO"`) {
		result.Level = "INFO"
	} else if strings.Contains(line, `"level":"WARN"`) {
		result.Level = "WARN"
	} else if strings.Contains(line, `"level":"ERROR"`) {
		result.Level = "ERROR"
	} else {
		result.Level = "INFO"
	}

	// 시간 추출
	if idx := strings.Index(line, `"date":"`); idx != -1 {
		start := idx + 8
		end := strings.Index(line[start:], `"`)
		if end > 0 {
			dateStr := line[start : start+end]
			// ISO8601에서 시간만 추출
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				result.Time = t.Format("15:04:05")
			} else {
				result.Time = dateStr
			}
		}
	}

	// 메시지 추출
	// "Info":"실제 메시지" 또는 "Debug":"메시지" 등
	for _, key := range []string{`"Info":"`, `"Debug":"`, `"Warn":"`, `"Err":"`} {
		if idx := strings.Index(line, key); idx != -1 {
			start := idx + len(key)
			end := strings.Index(line[start:], `"`)
			if end > 0 {
				result.Message = line[start : start+end]
				break
			}
		}
	}

	if result.Message == "" {
		// 메시지를 찾지 못하면 전체 라인 사용
		result.Message = line
		if len(result.Message) > 80 {
			result.Message = result.Message[:80] + "..."
		}
	}

	return result
}

// GetLines는 현재 로그 라인들 반환
func (lv *LogViewer) GetLines() []LogLine {
	return lv.lines
}

// Render는 로그 뷰어를 문자열로 렌더링
func (lv *LogViewer) Render(width int) string {
	var b strings.Builder

	title := fmt.Sprintf("LOGS [Node %d]", lv.nodeIndex+1)
	b.WriteString(styles.HeaderStyle.Render(title))
	b.WriteString("\n")

	if len(lv.lines) == 0 {
		b.WriteString(styles.MutedStyle.Render("  로그가 없습니다"))
		return b.String()
	}

	for _, line := range lv.lines {
		levelStyle := styles.LogLevelStyle(line.Level)

		timeStr := line.Time
		if timeStr == "" {
			timeStr = "        "
		}

		levelStr := fmt.Sprintf("%-5s", line.Level)

		msg := line.Message
		maxMsgLen := width - 20
		if maxMsgLen < 20 {
			maxMsgLen = 20
		}
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen] + "..."
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			styles.MutedStyle.Render(timeStr),
			levelStyle.Render(levelStr),
			msg))
	}

	return b.String()
}
