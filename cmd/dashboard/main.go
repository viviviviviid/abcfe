package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/abcfe/abcfe-node/internal/dashboard"
	"github.com/spf13/cobra"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"

	host    string
	ports   string
	logDir  string
	refresh int
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "abcfe-dashboard",
		Short: "ABCFe 노드 모니터링 대시보드",
		Long: `ABCFe Dashboard - 멀티 노드 실시간 모니터링 TUI

최대 9개 노드의 상태, 컨센서스, 로그를 한 화면에서 모니터링합니다.

사용 예시:
  abcfe-dashboard                           # localhost:8000-8009 스캔
  abcfe-dashboard --ports 8000,8001,8002    # 특정 포트만
  abcfe-dashboard --host 192.168.1.100      # 원격 호스트`,
		Run: func(cmd *cobra.Command, args []string) {
			runDashboard()
		},
	}

	rootCmd.Flags().StringVar(&host, "host", "localhost", "노드 호스트 주소")
	rootCmd.Flags().StringVar(&ports, "ports", "", "모니터링할 포트 (쉼표 구분, 예: 8000,8001,8002)")
	rootCmd.Flags().StringVar(&logDir, "log-dir", "./log", "로그 디렉토리 경로")
	rootCmd.Flags().IntVar(&refresh, "refresh", 1, "새로고침 간격 (초)")

	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "버전 정보 출력",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ABCFe Dashboard v%s (built: %s)\n", Version, BuildTime)
		},
	}
}

func runDashboard() {
	// 포트 파싱
	var portList []int
	if ports == "" {
		// 기본: 8000-8009 스캔
		for i := 8000; i <= 8009; i++ {
			portList = append(portList, i)
		}
	} else {
		for _, p := range strings.Split(ports, ",") {
			var port int
			if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &port); err == nil {
				portList = append(portList, port)
			}
		}
	}

	if len(portList) == 0 {
		fmt.Println("Error: 유효한 포트가 없습니다")
		os.Exit(1)
	}

	config := dashboard.Config{
		Host:       host,
		Ports:      portList,
		LogDir:     logDir,
		RefreshSec: refresh,
	}

	if err := dashboard.Run(config); err != nil {
		fmt.Printf("Dashboard error: %v\n", err)
		os.Exit(1)
	}
}
