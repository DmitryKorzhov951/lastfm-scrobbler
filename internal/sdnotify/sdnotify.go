//go:build linux

package sdnotify

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	conn *net.UnixConn
	done bool
)

func dial() *net.UnixConn {
	mu.Lock()
	defer mu.Unlock()
	if done {
		return conn
	}
	done = true

	addr := os.Getenv("NOTIFY_SOCKET")
	if addr == "" {
		return nil
	}
	if addr[0] == '@' {
		addr = "\x00" + addr[1:]
	}
	c, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		return nil
	}
	conn = c
	return conn
}

func Enabled() bool { return dial() != nil }

func send(msg string) {
	c := dial()
	if c == nil {
		return
	}
	_, _ = c.Write([]byte(msg))
}

func Ready() { send("READY=1") }

func Status(format string, args ...any) {
	send("STATUS=" + fmt.Sprintf(format, args...))
}

func Stopping() { send("STOPPING=1") }

func Watchdog() { send("WATCHDOG=1") }

func WatchdogInterval() time.Duration {
	v := os.Getenv("WATCHDOG_USEC")
	if v == "" {
		return 0
	}
	if pid := os.Getenv("WATCHDOG_PID"); pid != "" {
		if n, err := strconv.Atoi(pid); err == nil && n != os.Getpid() {
			return 0
		}
	}
	usec, err := strconv.ParseInt(v, 10, 64)
	if err != nil || usec <= 0 {
		return 0
	}
	return time.Duration(usec) * time.Microsecond / 2
}
