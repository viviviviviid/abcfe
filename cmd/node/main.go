package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/abcfe/abcfe-node/app"
	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/spf13/cobra"
)

// 버전 정보 (Makefile에서 주입됨)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// pid 관리 파일 - 사용자 홈 디렉토리 사용
func getPidFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// fallback to current directory
		return "./abcfe-node.pid"
	}
	return filepath.Join(homeDir, ".abcfe-node", "abcfe-node.pid")
}

var (
	pidFile = getPidFilePath()
)

var configFile string

func main() {
	var rootCmd = &cobra.Command{
		Use:   "abcfe-node",
		Short: "ABCFe blockchain node",
		Long:  `ABCFe blockchain node implementation that visualizes node interactions through REST API, WebSocket, and consensus protocols.`,
		Run: func(cmd *cobra.Command, args []string) {
			runNode()
		},
	}

	// 글로벌 플래그 등록
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file")

	rootCmd.AddCommand(nodeCmd())
	// rootCmd.AddCommand(walletCmd())
	// rootCmd.AddCommand(configCmd())
	// rootCmd.AddCommand(debugCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Failed to execute command:", err)
		os.Exit(1)
	}
}

func nodeCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "node",
		Short: "Node management commands",
		Long:  `Commands for managing the blockchain node.`,
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the node as daemon",
		Run: func(cmd *cobra.Command, args []string) {
			runNodeDaemon(pidFile)
		},
	}
	cmd.AddCommand(startCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop the node",
		Run: func(cmd *cobra.Command, args []string) {
			stopDaemon(pidFile)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show node status",
		Run: func(cmd *cobra.Command, args []string) {
			showStatus(pidFile)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "restart",
		Short: "Restart the node",
		Run: func(cmd *cobra.Command, args []string) {
			restartDaemon(pidFile)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show detailed node information",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("Node ID: abc123")
			logger.Info("Network: mainnet")
			logger.Info("Block height: 12345")
		},
	})

	return cmd
}

func runNode() {
	application, err := app.New(configFile)
	if err != nil {
		fmt.Println("Failed to initialize application:", err)
		os.Exit(1)
	}

	application.SigHandler()
	logger.Info("Node start.")

	// REST API, P2P, Consensus 모두 시작
	if err := application.StartAll(); err != nil {
		logger.Error("Failed to start services:", err)
		application.Terminate()
		os.Exit(1)
	}

	application.Wait()
	logger.Info("Node terminated.")
}

// 데몬으로 시작 - logger 에러 처리 개선
func runNodeDaemon(pidFilePath string) {
	// 환경변수로 내부 실행인지 확인 (무한 재귀 방지)
	if os.Getenv("ABCFE_DAEMON_CHILD") == "1" {
		// 실제 노드 실행 (자식 프로세스)
		runNode()
		return
	}

	// 이미 실행 중인지 확인
	if isRunning(pidFilePath) {
		fmt.Println("Node is already running")
		return
	}

	// 현재 실행 파일 경로 가져오기
	executable, err := os.Executable()
	if err != nil {
		// logger가 초기화되지 않았을 수 있으므로 fmt 사용
		fmt.Printf("Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(executable, "node", "start")
	cmd.Env = append(os.Environ(), "ABCFE_DAEMON_CHILD=1")

	// 표준 입출력을 null로 리다이렉트 (완전한 데몬화)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// 새로운 프로세스 그룹으로 시작
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// 백그라운드 프로세스 시작
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	// PID 파일 생성
	if err := writePidFile(pidFilePath, cmd.Process.Pid); err != nil {
		fmt.Printf("Failed to write PID file: %v\n", err)
		cmd.Process.Kill()
		os.Exit(1)
	}

	fmt.Printf("Node started as daemon with PID %d\n", cmd.Process.Pid)

	// 부모 프로세스는 여기서 종료
	os.Exit(0)
}

func stopDaemon(pidFilePath string) {
	pid, err := readPidFile(pidFilePath)
	if err != nil {
		fmt.Println("Node is not running or PID file not found")
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Process not found")
		removePidFile(pidFilePath)
		return
	}

	// SIGTERM 신호 전송
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to stop process: %v\n", err)
		return
	}

	fmt.Printf("Stopping node (PID: %d)...\n", pid)

	// PID 파일 제거
	removePidFile(pidFilePath)
}

func restartDaemon(pidFilePath string) {
	fmt.Println("Restarting node...")
	stopDaemon(pidFilePath)

	// 잠시 대기
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000000000) // 5초
	defer cancel()

	select {
	case <-ctx.Done():
		// 타임아웃 후 다시 시작
		runNodeDaemon(pidFilePath)
	}
}

// 상태 확인
func showStatus(pidFilePath string) {
	fmt.Printf("PID file path: %s\n", pidFilePath)

	if isRunning(pidFilePath) {
		pid, _ := readPidFile(pidFilePath)
		fmt.Printf("Node is running (PID: %d)\n", pid)
	} else {
		fmt.Println("Node is not running")

		// PID 파일이 존재하는지 확인
		if _, err := os.Stat(pidFilePath); err == nil {
			fmt.Println("PID file exists but process is not running - cleaning up")
			removePidFile(pidFilePath)
		}
	}
}

// 실행 중인지 확인
func isRunning(pidFilePath string) bool {
	pid, err := readPidFile(pidFilePath)
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 프로세스가 실제로 살아있는지 확인 (Unix/Linux에서)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// PID 파일 읽기
func readPidFile(pidFilePath string) (int, error) {
	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// PID 파일 쓰기
func writePidFile(pidFilePath string, pid int) error {
	// 디렉토리가 존재하지 않으면 생성
	dir := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644)
}

// PID 파일 제거
func removePidFile(pidFilePath string) {
	os.Remove(pidFilePath)
}

// 개선된 runNode 함수 - 신호 처리 포함
func runNodeWithSignalHandling() {
	// 신호 채널 생성
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	application, err := app.New(configFile)
	if err != nil {
		fmt.Println("Failed to initialize application:", err)
		os.Exit(1)
	}

	application.SigHandler()
	logger.Info("Node start.")

	// REST API, P2P, Consensus 모두 시작
	if err := application.StartAll(); err != nil {
		logger.Error("Failed to start services:", err)
		application.Terminate()
		os.Exit(1)
	}

	// 고루틴에서 신호 대기
	go func() {
		sig := <-sigChan
		logger.Info("Received signal:", sig)

		// 정리 작업
		application.Terminate()

		// PID 파일 제거 (데몬 모드인 경우)
		removePidFile(pidFile)

		os.Exit(0)
	}()

	application.Wait()
	logger.Info("Node terminated.")
}
