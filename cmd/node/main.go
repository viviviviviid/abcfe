package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/abcfe/abcfe-node/app"
	"github.com/abcfe/abcfe-node/common/logger"
	"github.com/abcfe/abcfe-node/wallet"
	"github.com/spf13/cobra"
)

// Version info (Injected from Makefile)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// PID file management - Use user home directory
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

	// Register global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file")

	rootCmd.AddCommand(nodeCmd())
	rootCmd.AddCommand(walletCmd())
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

	// Start all services: REST API, P2P, Consensus
	if err := application.StartAll(); err != nil {
		logger.Error("Failed to start services:", err)
		application.Terminate()
		os.Exit(1)
	}

	application.Wait()
	logger.Info("Node terminated.")
}

// Start as daemon - improved logger error handling
func runNodeDaemon(pidFilePath string) {
	// Check internal execution via env var (prevent infinite recursion)
	if os.Getenv("ABCFE_DAEMON_CHILD") == "1" {
		// Execute actual node (Child process)
		runNode()
		return
	}

	// Check if already running
	if isRunning(pidFilePath) {
		fmt.Println("Node is already running")
		return
	}

	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		// Use fmt as logger might not be initialized
		fmt.Printf("Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(executable, "node", "start")
	cmd.Env = append(os.Environ(), "ABCFE_DAEMON_CHILD=1")

	// Redirect standard I/O to null (Complete daemonization)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start in a new process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start background process
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	// Create PID file
	if err := writePidFile(pidFilePath, cmd.Process.Pid); err != nil {
		fmt.Printf("Failed to write PID file: %v\n", err)
		cmd.Process.Kill()
		os.Exit(1)
	}

	fmt.Printf("Node started as daemon with PID %d\n", cmd.Process.Pid)

	// Parent process exits here
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

	// Send SIGTERM signal
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to stop process: %v\n", err)
		return
	}

	fmt.Printf("Stopping node (PID: %d)...\n", pid)

	// Remove PID file
	removePidFile(pidFilePath)
}

func restartDaemon(pidFilePath string) {
	fmt.Println("Restarting node...")
	stopDaemon(pidFilePath)

	// Wait briefly
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000000000) // 5초
	defer cancel()

	select {
	case <-ctx.Done():
		// Restart after timeout
		runNodeDaemon(pidFilePath)
	}
}

// Check status
func showStatus(pidFilePath string) {
	fmt.Printf("PID file path: %s\n", pidFilePath)

	if isRunning(pidFilePath) {
		pid, _ := readPidFile(pidFilePath)
		fmt.Printf("Node is running (PID: %d)\n", pid)
	} else {
		fmt.Println("Node is not running")

		// Check if PID file exists
		if _, err := os.Stat(pidFilePath); err == nil {
			fmt.Println("PID file exists but process is not running - cleaning up")
			removePidFile(pidFilePath)
		}
	}
}

// Check if running
func isRunning(pidFilePath string) bool {
	pid, err := readPidFile(pidFilePath)
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Check if process is actually alive (Unix/Linux)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Read PID file
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

// Write PID file
func writePidFile(pidFilePath string, pid int) error {
	// Create directory if not exists
	dir := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644)
}

// Remove PID file
func removePidFile(pidFilePath string) {
	os.Remove(pidFilePath)
}

// Improved runNode function - includes signal handling
func runNodeWithSignalHandling() {
	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	application, err := app.New(configFile)
	if err != nil {
		fmt.Println("Failed to initialize application:", err)
		os.Exit(1)
	}

	application.SigHandler()
	logger.Info("Node start.")

	// Start all services: REST API, P2P, Consensus
	if err := application.StartAll(); err != nil {
		logger.Error("Failed to start services:", err)
		application.Terminate()
		os.Exit(1)
	}

	// Wait for signal in goroutine
	go func() {
		sig := <-sigChan
		logger.Info("Received signal:", sig)

		// Cleanup task
		application.Terminate()

		// Remove PID file (if in daemon mode)
		removePidFile(pidFile)

		os.Exit(0)
	}()

	application.Wait()
	logger.Info("Node terminated.")
}

// Default wallet directory path
func getDefaultWalletDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./wallets"
	}
	return filepath.Join(homeDir, ".abcfe-node", "wallets")
}

var walletDir string

func walletCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wallet",
		Short: "Wallet management commands",
		Long:  `Commands for managing wallets, accounts, and keys.`,
	}

	// Wallet directory flag
	cmd.PersistentFlags().StringVarP(&walletDir, "wallet-dir", "w", getDefaultWalletDir(), "Wallet directory path")

	// Add subcommands
	cmd.AddCommand(walletCreateCmd())
	cmd.AddCommand(walletRestoreCmd())
	cmd.AddCommand(walletListCmd())
	cmd.AddCommand(walletAddAccountCmd())
	cmd.AddCommand(walletShowMnemonicCmd())

	return cmd
}

// Create new wallet
func walletCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a new wallet with mnemonic",
		Run: func(cmd *cobra.Command, args []string) {
			wm := wallet.NewWalletManager(walletDir)

			// Check existing wallet
			walletFile := filepath.Join(walletDir, "wallet.json")
			if _, err := os.Stat(walletFile); err == nil {
				fmt.Println("Wallet already exists at:", walletFile)
				fmt.Println("Use 'wallet restore' to restore from mnemonic or delete the existing wallet first.")
				return
			}

			// Create new wallet
			mnemonicWallet, err := wm.CreateWallet()
			if err != nil {
				fmt.Printf("Failed to create wallet: %v\n", err)
				return
			}

			// Save wallet
			if err := wm.SaveWallet(); err != nil {
				fmt.Printf("Failed to save wallet: %v\n", err)
				return
			}

			fmt.Println("=== New Wallet Created ===")
			fmt.Println("")
			fmt.Println("IMPORTANT: Write down your mnemonic phrase and keep it safe!")
			fmt.Println("If you lose it, you will lose access to your wallet forever.")
			fmt.Println("")
			fmt.Printf("Mnemonic: %s\n", mnemonicWallet.Mnemonic)
			fmt.Println("")
			fmt.Println("=== First Account ===")
			if len(mnemonicWallet.Accounts) > 0 {
				account := mnemonicWallet.Accounts[0]
				fmt.Printf("Address: %s\n", hex.EncodeToString(account.Address[:]))
				fmt.Printf("Path: %s\n", account.Path)
			}
			fmt.Println("")
			fmt.Printf("Wallet saved to: %s\n", walletFile)
		},
	}
}

// Restore wallet from mnemonic
func walletRestoreCmd() *cobra.Command {
	var mnemonic string

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore wallet from mnemonic",
		Run: func(cmd *cobra.Command, args []string) {
			if mnemonic == "" {
				fmt.Println("Please provide a mnemonic phrase with --mnemonic flag")
				return
			}

			wm := wallet.NewWalletManager(walletDir)

			// Check existing wallet
			walletFile := filepath.Join(walletDir, "wallet.json")
			if _, err := os.Stat(walletFile); err == nil {
				fmt.Println("Wallet already exists at:", walletFile)
				fmt.Println("Please delete the existing wallet first if you want to restore.")
				return
			}

			// Restore wallet
			mnemonicWallet, err := wm.RestoreWallet(mnemonic)
			if err != nil {
				fmt.Printf("Failed to restore wallet: %v\n", err)
				return
			}

			// Save wallet
			if err := wm.SaveWallet(); err != nil {
				fmt.Printf("Failed to save wallet: %v\n", err)
				return
			}

			fmt.Println("=== Wallet Restored ===")
			fmt.Println("")
			if len(mnemonicWallet.Accounts) > 0 {
				account := mnemonicWallet.Accounts[0]
				fmt.Printf("Address: %s\n", hex.EncodeToString(account.Address[:]))
				fmt.Printf("Path: %s\n", account.Path)
			}
			fmt.Println("")
			fmt.Printf("Wallet saved to: %s\n", walletFile)
		},
	}

	cmd.Flags().StringVarP(&mnemonic, "mnemonic", "m", "", "Mnemonic phrase to restore")
	return cmd
}

// List accounts
func walletListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all accounts in the wallet",
		Run: func(cmd *cobra.Command, args []string) {
			wm := wallet.NewWalletManager(walletDir)

			if err := wm.LoadWalletFile(); err != nil {
				fmt.Printf("Failed to load wallet: %v\n", err)
				fmt.Println("Use 'wallet create' to create a new wallet.")
				return
			}

			accounts, err := wm.GetAccounts()
			if err != nil {
				fmt.Printf("Failed to get accounts: %v\n", err)
				return
			}

			fmt.Println("=== Wallet Accounts ===")
			fmt.Println("")
			for i, account := range accounts {
				current := ""
				if i == wm.Wallet.CurrentIndex {
					current = " (current)"
				}
				fmt.Printf("[%d]%s\n", i, current)
				fmt.Printf("  Address: %s\n", hex.EncodeToString(account.Address[:]))
				fmt.Printf("  Path: %s\n", account.Path)
				fmt.Println("")
			}
		},
	}
}

// Add new account
func walletAddAccountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add-account",
		Short: "Add a new account to the wallet",
		Run: func(cmd *cobra.Command, args []string) {
			wm := wallet.NewWalletManager(walletDir)

			if err := wm.LoadWalletFile(); err != nil {
				fmt.Printf("Failed to load wallet: %v\n", err)
				fmt.Println("Use 'wallet create' to create a new wallet first.")
				return
			}

			account, err := wm.AddAccount()
			if err != nil {
				fmt.Printf("Failed to add account: %v\n", err)
				return
			}

			// Save wallet
			if err := wm.SaveWallet(); err != nil {
				fmt.Printf("Failed to save wallet: %v\n", err)
				return
			}

			fmt.Println("=== New Account Added ===")
			fmt.Println("")
			fmt.Printf("Index: %d\n", account.Index)
			fmt.Printf("Address: %s\n", hex.EncodeToString(account.Address[:]))
			fmt.Printf("Path: %s\n", account.Path)
		},
	}
}

// Show mnemonic
func walletShowMnemonicCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show-mnemonic",
		Short: "Show the wallet's mnemonic phrase",
		Run: func(cmd *cobra.Command, args []string) {
			wm := wallet.NewWalletManager(walletDir)

			if err := wm.LoadWalletFile(); err != nil {
				fmt.Printf("Failed to load wallet: %v\n", err)
				return
			}

			mnemonic, err := wm.GetMnemonic()
			if err != nil {
				fmt.Printf("Failed to get mnemonic: %v\n", err)
				return
			}

			fmt.Println("=== Wallet Mnemonic ===")
			fmt.Println("")
			fmt.Println("WARNING: Never share your mnemonic with anyone!")
			fmt.Println("")
			fmt.Printf("Mnemonic: %s\n", mnemonic)
		},
	}
}
