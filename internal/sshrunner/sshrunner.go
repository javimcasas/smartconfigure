package sshrunner

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/report"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// Config controls timing behaviour of the interactive session. Network
// devices don't reliably signal "command finished" the way a Unix shell
// does, so v1 uses an idle-based heuristic: after sending a line, keep
// reading whatever the device sends back until it stays quiet for
// IdleTimeout, then move on to the next line. LineTimeout is a hard cap so
// a stuck device (or a command awaiting a y/n confirmation we didn't
// anticipate) doesn't hang the whole run forever.
type Config struct {
	Port           int
	ConnectTimeout time.Duration
	IdleTimeout    time.Duration
	LineTimeout    time.Duration
}

// Run opens an SSH session to dev, sends every line of the template (with
// {VARIABLE} placeholders substituted from dev.Vars), and writes the full
// raw transcript to logPath. It never returns a Go error directly —
// failures are reported through the returned report.Result so the caller
// can keep going with the rest of the batch instead of aborting everything.
func Run(dev excelsheet.Device, lines []string, cfg Config, logPath string) report.Result {
	res := report.Result{Device: dev.IP}

	logFile, ferr := os.Create(logPath)
	if ferr != nil {
		res.Error = fmt.Sprintf("could not create log file: %v", ferr)
		return res
	}
	defer logFile.Close()

	writeLog := func(format string, args ...interface{}) {
		fmt.Fprintf(logFile, format, args...)
	}

	sshCfg := &ssh.ClientConfig{
		User:            dev.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(dev.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // network gear rarely has a known_hosts entry on first bring-up
		Timeout:         cfg.ConnectTimeout,
	}

	addr := net.JoinHostPort(dev.IP, portString(cfg.Port))
	writeLog("=== Connecting to %s ===\n", addr)

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		res.Error = fmt.Sprintf("SSH connection failed: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		res.Error = fmt.Sprintf("could not open session: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}
	defer session.Close()

	if err := session.RequestPty("xterm", 200, 200, ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		res.Error = fmt.Sprintf("could not request pty: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		res.Error = fmt.Sprintf("could not open stdin: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		res.Error = fmt.Sprintf("could not open stdout: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}

	if err := session.Shell(); err != nil {
		res.Error = fmt.Sprintf("could not start shell: %v", err)
		writeLog("ERROR: %s\n", res.Error)
		return res
	}

	// Drain whatever banner/prompt the device sends right after login.
	writeLog("%s", readUntilIdle(stdout, cfg.IdleTimeout, cfg.LineTimeout))

	for i, rawLine := range lines {
		line := template.Render(rawLine, dev.Vars)
		writeLog("\n>>> %s\n", line)

		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			res.Error = fmt.Sprintf("line %d (%q): write failed: %v", i+1, line, err)
			writeLog("ERROR: %s\n", res.Error)
			return res
		}

		output := readUntilIdle(stdout, cfg.IdleTimeout, cfg.LineTimeout)
		writeLog("%s", output)

		if looksLikeError(output) {
			res.Error = fmt.Sprintf("line %d (%q) looks like it errored — check the log", i+1, line)
			writeLog("\n=== Stopped: possible error detected, aborting remaining lines ===\n")
			return res
		}
	}

	res.Success = true
	writeLog("\n=== Finished: all %d line(s) sent ===\n", len(lines))
	return res
}

func portString(p int) string {
	if p <= 0 {
		p = 22
	}
	return fmt.Sprintf("%d", p)
}

// readUntilIdle reads from r until no new bytes arrive for `idle`, or until
// `hardCap` total time has passed since the call started, whichever comes
// first. It's a pragmatic v1 substitute for real prompt-regex matching.
func readUntilIdle(r io.Reader, idle, hardCap time.Duration) string {
	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	deadlineAt := time.Now().Add(hardCap)

	type readResult struct {
		n   int
		err error
	}
	resultCh := make(chan readResult, 1)

	readOnce := func() {
		n, err := r.Read(chunk)
		resultCh <- readResult{n, err}
	}
	go readOnce()

	for {
		timeout := idle
		if remaining := time.Until(deadlineAt); remaining < timeout {
			timeout = remaining
		}
		if timeout <= 0 {
			return buf.String()
		}

		select {
		case res := <-resultCh:
			if res.n > 0 {
				buf.Write(chunk[:res.n])
			}
			if res.err != nil {
				return buf.String()
			}
			go readOnce()
		case <-time.After(timeout):
			return buf.String()
		}
	}
}

// looksLikeError does a best-effort scan for common CLI error markers. This
// is intentionally conservative (few, specific phrases) so it doesn't
// false-positive on normal output; it exists to stop a run early instead of
// blindly sending the rest of the template into a broken session.
func looksLikeError(output string) bool {
	markers := []string{
		"% Unrecognized command",
		"% Wrong parameter",
		"Error: ",
		"% Invalid",
		"Unrecognized command found",
	}
	for _, m := range markers {
		if bytes.Contains([]byte(output), []byte(m)) {
			return true
		}
	}
	return false
}
