package styles

import "github.com/charmbracelet/lipgloss"

var (
	// 색상 정의
	Primary   = lipgloss.Color("#04B575")
	Secondary = lipgloss.Color("#3C3C3C")
	Success   = lipgloss.Color("#04B575")
	Warning   = lipgloss.Color("#FFCC00")
	Error     = lipgloss.Color("#FF5F56")
	Muted     = lipgloss.Color("#626262")
	White     = lipgloss.Color("#FFFFFF")
	Cyan      = lipgloss.Color("#00CED1")

	// 컨센서스 상태별 색상
	StateColors = map[string]lipgloss.Color{
		"IDLE":          Muted,
		"PROPOSING":     Cyan,
		"PREVOTING":     Warning,
		"PRECOMMITTING": lipgloss.Color("#FFA500"),
		"COMMITTING":    Success,
	}

	// 로그 레벨별 색상
	LogLevelColors = map[string]lipgloss.Color{
		"DEBUG": Muted,
		"INFO":  Cyan,
		"WARN":  Warning,
		"ERROR": Error,
	}

	// 기본 스타일
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(White).
			Background(Primary).
			Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Secondary).
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// 테이블 스타일
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(White).
				Background(Secondary).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableSelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(White).
				Background(Primary).
				Padding(0, 1)

	// 도움말 바 스타일
	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Muted)

	HelpBarStyle = lipgloss.NewStyle().
			Foreground(Muted).
			MarginTop(1)
)

// StateStyle 컨센서스 상태에 맞는 스타일 반환
func StateStyle(state string) lipgloss.Style {
	color, ok := StateColors[state]
	if !ok {
		color = Muted
	}
	return lipgloss.NewStyle().Foreground(color)
}

// LogLevelStyle 로그 레벨에 맞는 스타일 반환
func LogLevelStyle(level string) lipgloss.Style {
	color, ok := LogLevelColors[level]
	if !ok {
		color = White
	}
	return lipgloss.NewStyle().Foreground(color)
}
