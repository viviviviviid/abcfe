package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/abcfe/abcfe-node/internal/dashboard/api"
	"github.com/abcfe/abcfe-node/internal/dashboard/components"
	"github.com/abcfe/abcfe-node/internal/dashboard/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Config는 대시보드 설정
type Config struct {
	Host       string
	Ports      []int
	LogDir     string
	RefreshSec int
}

// NodeInfo는 노드 상태 정보
type NodeInfo struct {
	Port      int
	Online    bool
	Status    *api.NodeStatus
	Consensus *api.ConsensusStatus
	P2P       *api.P2PStatus
	Error     string
}

// Model은 Bubbletea 모델
type Model struct {
	config       Config
	nodes        []NodeInfo
	clients      []*api.Client
	selectedNode int
	maxHeight    uint64
	width        int
	height       int
	logViewer    *components.LogViewer
	showHelp     bool
	quitting     bool
}

// Run은 대시보드 실행
func Run(config Config) error {
	m := initialModel(config)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(config Config) Model {
	nodes := make([]NodeInfo, len(config.Ports))
	clients := make([]*api.Client, len(config.Ports))

	for i, port := range config.Ports {
		nodes[i] = NodeInfo{Port: port}
		clients[i] = api.NewClient(config.Host, port)
	}

	logViewer := components.NewLogViewer(config.LogDir, 10)

	return Model{
		config:       config,
		nodes:        nodes,
		clients:      clients,
		selectedNode: 0,
		logViewer:    logViewer,
	}
}

// tickMsg는 주기적 업데이트 메시지
type tickMsg time.Time

// nodeUpdateMsg는 노드 상태 업데이트 메시지
type nodeUpdateMsg struct {
	index     int
	status    *api.NodeStatus
	consensus *api.ConsensusStatus
	p2p       *api.P2PStatus
	err       error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(m.config.RefreshSec),
		m.fetchAllNodes(),
	)
}

func tickCmd(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchAllNodes() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.clients {
		cmds = append(cmds, m.fetchNode(i))
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchNode(index int) tea.Cmd {
	return func() tea.Msg {
		client := m.clients[index]

		status, err := client.GetStatus()
		if err != nil {
			return nodeUpdateMsg{index: index, err: err}
		}

		consensus, _ := client.GetConsensusStatus()
		p2p, _ := client.GetP2PStatus()

		return nodeUpdateMsg{
			index:     index,
			status:    status,
			consensus: consensus,
			p2p:       p2p,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp

		case "r":
			return m, m.fetchAllNodes()

		case "up", "k":
			if m.selectedNode > 0 {
				m.selectedNode--
				m.logViewer.SetNode(m.selectedNode)
			}

		case "down", "j":
			if m.selectedNode < len(m.nodes)-1 {
				m.selectedNode++
				m.logViewer.SetNode(m.selectedNode)
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(m.nodes) {
				m.selectedNode = idx
				m.logViewer.SetNode(m.selectedNode)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		cmds = append(cmds, tickCmd(m.config.RefreshSec))
		cmds = append(cmds, m.fetchAllNodes())
		m.logViewer.Refresh()

	case nodeUpdateMsg:
		if msg.index < len(m.nodes) {
			if msg.err != nil {
				m.nodes[msg.index].Online = false
				m.nodes[msg.index].Error = msg.err.Error()
			} else {
				m.nodes[msg.index].Online = true
				m.nodes[msg.index].Status = msg.status
				m.nodes[msg.index].Consensus = msg.consensus
				m.nodes[msg.index].P2P = msg.p2p
				m.nodes[msg.index].Error = ""
			}
			m.updateMaxHeight()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateMaxHeight() {
	m.maxHeight = 0
	for _, node := range m.nodes {
		if node.Online && node.Status != nil {
			if node.Status.CurrentHeight > m.maxHeight {
				m.maxHeight = node.Status.CurrentHeight
			}
		}
	}
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// 헤더
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// 노드 테이블
	b.WriteString(m.renderNodesTable())
	b.WriteString("\n")

	// 선택된 노드 상세 정보
	if m.selectedNode < len(m.nodes) {
		b.WriteString(m.renderNodeDetails(m.nodes[m.selectedNode]))
		b.WriteString("\n")
	}

	// 로그 뷰어
	b.WriteString(m.logViewer.Render(m.width))
	b.WriteString("\n")

	// 도움말 또는 단축키 바
	if m.showHelp {
		b.WriteString(m.renderFullHelp())
	} else {
		b.WriteString(m.renderHelpBar())
	}

	return b.String()
}

func (m Model) renderHeader() string {
	title := styles.TitleStyle.Render(" ABCFe Dashboard v1.0.0 ")

	onlineCount := 0
	for _, n := range m.nodes {
		if n.Online {
			onlineCount++
		}
	}

	status := fmt.Sprintf("노드: %d/%d 온라인", onlineCount, len(m.nodes))
	if m.maxHeight > 0 {
		status += fmt.Sprintf(" | 최고 높이: %d", m.maxHeight)
	}

	statusText := styles.MutedStyle.Render(status)

	// 오른쪽 정렬
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(statusText) - 2
	if gap < 1 {
		gap = 1
	}

	return title + strings.Repeat(" ", gap) + statusText
}

func (m Model) renderNodesTable() string {
	var b strings.Builder

	// 테이블 헤더
	header := fmt.Sprintf("%-4s %-6s %-10s %-6s %-14s %-8s %-8s",
		"#", "Port", "Height", "Peers", "State", "Proposer", "Sync")
	b.WriteString(styles.TableHeaderStyle.Render(header))
	b.WriteString("\n")

	// 노드 행
	for i, node := range m.nodes {
		row := m.renderNodeRow(i, node)
		if i == m.selectedNode {
			b.WriteString(styles.TableSelectedRowStyle.Render(row))
		} else {
			b.WriteString(styles.TableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderNodeRow(index int, node NodeInfo) string {
	num := fmt.Sprintf("%d", index+1)
	port := fmt.Sprintf("%d", node.Port)

	if !node.Online {
		return fmt.Sprintf("%-4s %-6s %-10s %-6s %-14s %-8s %-8s",
			num, port, "OFFLINE", "-", "-", "-", "-")
	}

	height := "-"
	peers := "-"
	state := "-"
	proposer := ""
	sync := "✓"

	if node.Status != nil {
		height = fmt.Sprintf("%d", node.Status.CurrentHeight)

		// 동기화 상태
		if m.maxHeight > 0 && node.Status.CurrentHeight < m.maxHeight {
			diff := m.maxHeight - node.Status.CurrentHeight
			sync = fmt.Sprintf("⚠ -%d", diff)
		}
	}

	if node.P2P != nil {
		peers = fmt.Sprintf("%d", node.P2P.PeerCount)
	}

	if node.Consensus != nil {
		state = node.Consensus.State

		// 현재 제안자인지 확인
		if node.Consensus.Proposer != "" && node.Status != nil {
			proposer = "●"
		}
	}

	return fmt.Sprintf("%-4s %-6s %-10s %-6s %-14s %-8s %-8s",
		num, port, height, peers, state, proposer, sync)
}

func (m Model) renderNodeDetails(node NodeInfo) string {
	var b strings.Builder

	title := fmt.Sprintf("Node %d Details (Port: %d)", m.selectedNode+1, node.Port)
	b.WriteString(styles.HeaderStyle.Render(title))
	b.WriteString("\n")

	if !node.Online {
		b.WriteString(styles.ErrorStyle.Render("  ✗ 오프라인"))
		if node.Error != "" {
			b.WriteString("\n")
			b.WriteString(styles.MutedStyle.Render("  " + node.Error))
		}
		return b.String()
	}

	// 컨센서스 정보
	if node.Consensus != nil {
		stateStyle := styles.StateStyle(node.Consensus.State)
		b.WriteString(fmt.Sprintf("  State: %s", stateStyle.Render(node.Consensus.State)))
		b.WriteString(fmt.Sprintf("  Round: %d", node.Consensus.CurrentRound))

		if node.Consensus.Proposer != "" {
			proposer := node.Consensus.Proposer
			if len(proposer) > 16 {
				proposer = proposer[:16] + "..."
			}
			b.WriteString(fmt.Sprintf("  Proposer: %s", proposer))
		}

		activeCount := 0
		for _, v := range node.Consensus.Validators {
			if v.IsActive {
				activeCount++
			}
		}
		b.WriteString(fmt.Sprintf("  Validators: %d/%d", activeCount, len(node.Consensus.Validators)))
	}

	// 블록 해시
	if node.Status != nil && node.Status.CurrentBlockHash != "" {
		hash := node.Status.CurrentBlockHash
		if len(hash) > 16 {
			hash = hash[:16] + "..."
		}
		b.WriteString("\n")
		b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("  Block: %s  Mempool: %d txs",
			hash, node.Status.MempoolSize)))
	}

	return b.String()
}

func (m Model) renderHelpBar() string {
	keys := []struct{ key, desc string }{
		{"↑↓", "선택"},
		{"1-9", "노드 전환"},
		{"r", "새로고침"},
		{"?", "도움말"},
		{"q", "종료"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts,
			styles.HelpKeyStyle.Render(k.key)+
				styles.HelpDescStyle.Render(" "+k.desc))
	}

	return styles.HelpBarStyle.Render(strings.Join(parts, "  │  "))
}

func (m Model) renderFullHelp() string {
	help := `
╭─────────────────────────────────────╮
│            도움말                    │
├─────────────────────────────────────┤
│  ↑/↓, j/k    노드 선택 이동          │
│  1-9         노드 직접 선택          │
│  r           수동 새로고침           │
│  ?           도움말 토글             │
│  q, Ctrl+C   종료                   │
╰─────────────────────────────────────╯`
	return styles.MutedStyle.Render(help)
}
