// Package overseer implements daemonizable
// self-upgrading binaries in Go (golang).
package overseer

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/menglh/overseer/fetcher"
)

const (
	envSlaveID        = "OVERSEER_SLAVE_ID"
	envIsSlave        = "OVERSEER_IS_SLAVE"
	envNumFDs         = "OVERSEER_NUM_FDS"
	envBinID          = "OVERSEER_BIN_ID"
	envBinPath        = "OVERSEER_BIN_PATH"
	envBinCheck       = "OVERSEER_BIN_CHECK"
	envBinCheckLegacy = "GO_UPGRADE_BIN_CHECK"
)

// Config defines overseer's run-time configuration
type Config struct {
	//Required 将防止监督者在失败时回退到运行在主进程中运行程序。
	Required bool
	//Program's main function
	Program func(state State)
	//程序的零停机套接字侦听地址（设置此地址或地址）
	Address string
	//程序的零停机套接字侦听地址（设置此地址或地址）
	Addresses []string
	//RestartSignal 将手动触发正常重启。默认值为 SIGUSR2。
	RestartSignal os.Signal
	//TerminateTimeout 控制监督程序应等待程序自行终止的时间。在此超时之后，监督者将发出 SIGKILL。
	TerminateTimeout time.Duration
	//MinFetchInterval 定义 Fetch（） 之间的最小持续时间。
	//这有助于防止难以提取。占用太多资源的接口。默认值为 1 秒。
	MinFetchInterval time.Duration
	//PreUpgrade 在检索到二进制文件后运行，可以在此处运行用户定义的检查，返回错误将取消升级。
	PreUpgrade func(tempBinaryPath string) error
	//Debug enables all [overseer] logs.
	Debug bool
	//NoWarn disables warning [overseer] logs.
	NoWarn bool
	//NoRestart 禁用所有重启，此选项实质上是将 RestartSignal 转换为“ShutdownSignal”。
	NoRestart bool
	//NoRestartAfterFetch disables automatic restarts after each upgrade.
	//Though manual restarts using the RestartSignal can still be performed.
	NoRestartAfterFetch bool
	//Fetcher will be used to fetch binaries.
	Fetcher fetcher.Interface
}

func validate(c *Config) error {
	//validate
	if c.Program == nil {
		return errors.New("overseer.Config.Program required")
	}
	if c.Address != "" {
		if len(c.Addresses) > 0 {
			return errors.New("overseer.Config.Address and Addresses cant both be set")
		}
		c.Addresses = []string{c.Address}
	} else if len(c.Addresses) > 0 {
		c.Address = c.Addresses[0]
	}
	if c.RestartSignal == nil {
		c.RestartSignal = SIGUSR2
	}
	if c.TerminateTimeout <= 0 {
		c.TerminateTimeout = 30 * time.Second
	}
	if c.MinFetchInterval <= 0 {
		c.MinFetchInterval = 1 * time.Second
	}
	return nil
}

// RunErr allows manual handling of any
// overseer errors.
func RunErr(c Config) error {
	return runErr(&c)
}

// Run executes overseer, if an error is
// encountered, overseer fallsback to running
// the program directly (unless Required is set).
func Run(c Config) {
	err := runErr(&c)
	if err != nil {
		if c.Required {
			log.Fatalf("[overseer] %s", err)
		} else if c.Debug || !c.NoWarn {
			log.Printf("[overseer] disabled. run failed: %s", err)
		}
		c.Program(DisabledState)
		return
	}
	os.Exit(0)
}

// sanityCheck returns true if a check was performed
func sanityCheck() bool {
	//sanity check
	if token := os.Getenv(envBinCheck); token != "" {
		fmt.Fprint(os.Stdout, token)
		return true
	}
	//legacy sanity check using old env var
	if token := os.Getenv(envBinCheckLegacy); token != "" {
		fmt.Fprint(os.Stdout, token)
		return true
	}
	return false
}

// SanityCheck manually runs the check to ensure this binary
// is compatible with overseer. This tries to ensure that a restart
// is never performed against a bad binary, as it would require
// manual intervention to rectify. This is automatically done
// on overseer.Run() though it can be manually run prior whenever
// necessary.
func SanityCheck() {
	if sanityCheck() {
		os.Exit(0)
	}
}

// abstraction over master/slave
var currentProcess interface {
	triggerRestart()
	run() error
}

func runErr(c *Config) error {
	//os not supported
	if !supported {
		return fmt.Errorf("os (%s) not supported", runtime.GOOS)
	}
	if err := validate(c); err != nil {
		return err
	}
	if sanityCheck() {
		return nil
	}
	//run either in master or slave mode
	if os.Getenv(envIsSlave) == "1" {
		currentProcess = &slave{Config: c}
	} else {
		currentProcess = &master{Config: c}
	}
	return currentProcess.run()
}

// Restart programmatically triggers a graceful restart. If NoRestart
// is enabled, then this will essentially be a graceful shutdown.
func Restart() {
	if currentProcess != nil {
		currentProcess.triggerRestart()
	}
}

// IsSupported returns whether overseer is supported on the current OS.
func IsSupported() bool {
	return supported
}
