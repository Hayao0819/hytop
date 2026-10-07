//go:build !windows

package page

import (
	"syscall"
)

var Signals = []signalChoice{
	{"SIGTERM", processSignal(syscall.SIGTERM), "ask it to stop"},
	{"SIGINT", processSignal(syscall.SIGINT), "as if Ctrl+C"},
	{"SIGHUP", processSignal(syscall.SIGHUP), "reload, for a daemon"},
	{"SIGSTOP", processSignal(syscall.SIGSTOP), "freeze it"},
	{"SIGCONT", processSignal(syscall.SIGCONT), "let it run again"},
	{"SIGKILL", processSignal(syscall.SIGKILL), "take it away"},
}
