package sshrunner

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
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

const separator = "─────────────────────────────────────────────────────────────"

// Run opens an SSH session to dev, sends every line of the template (with
// {VARIABLE} placeholders substituted from dev.Vars), and writes the full,
// cleaned-up conversation transcript to logPath: nothing the device said is
// dropped, but line endings are normalized and terminal control sequences
// are stripped so the file reads like a real transcript instead of a raw
// terminal dump. It never returns a Go error directly — failures are
// reported through the returned report.Result so the caller can keep going
// with the rest of the batch instead of aborting everything.
func Run(dev excelsheet.Device, lines []string, cfg Config, logPath string) report.Result {
	res := report.Result{Device: dev.IP}
	runStart := time.Now()

	logFile, ferr := os.Create(logPath)
	if ferr != nil {
		res.Error = fmt.Sprintf("could not create log file: %v", ferr)
		return res
	}
	defer logFile.Close()

	writeHeader(logFile, dev, cfg, len(lines))

	sshCfg := &ssh.ClientConfig{
		User:            dev.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(dev.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // network gear rarely has a known_hosts entry on first bring-up
		Timeout:         cfg.ConnectTimeout,
	}

	addr := net.JoinHostPort(dev.IP, portString(cfg.Port))
	writeSection(logFile, "CONNECTING to "+addr, "")

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		res.Error = fmt.Sprintf("SSH connection failed: %v", err)
		writeSection(logFile, "CONNECTION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		res.Error = fmt.Sprintf("could not open session: %v", err)
		writeSection(logFile, "SESSION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}
	defer session.Close()

	if err := session.RequestPty("xterm", 200, 200, ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		res.Error = fmt.Sprintf("could not request pty: %v", err)
		writeSection(logFile, "SESSION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		res.Error = fmt.Sprintf("could not open stdin: %v", err)
		writeSection(logFile, "SESSION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		res.Error = fmt.Sprintf("could not open stdout: %v", err)
		writeSection(logFile, "SESSION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}

	if err := session.Shell(); err != nil {
		res.Error = fmt.Sprintf("could not start shell: %v", err)
		writeSection(logFile, "SESSION FAILED", res.Error)
		writeFooter(logFile, res, runStart)
		return res
	}

	// Drain whatever banner/prompt the device sends right after login.
	writeSection(logFile, "LOGIN BANNER", cleanOutput(readUntilIdle(stdout, cfg.IdleTimeout, cfg.LineTimeout)))

	for i, rawLine := range lines {
		line := template.Render(rawLine, dev.Vars)
		title := fmt.Sprintf("[%d/%d]  %s", i+1, len(lines), line)

		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			res.Error = fmt.Sprintf("line %d (%q): write failed: %v", i+1, line, err)
			writeSection(logFile, title, "ERROR sending this line: "+err.Error())
			writeFooter(logFile, res, runStart)
			return res
		}

		rawOutput := readUntilIdle(stdout, cfg.IdleTimeout, cfg.LineTimeout)
		output := cleanOutput(rawOutput)
		writeSection(logFile, title, output)

		if looksLikeError(output) {
			res.Error = fmt.Sprintf("line %d (%q) looks like it errored — check the log", i+1, line)
			writeSection(logFile, "STOPPED", "Possible error detected in the device's response above. The rest of the template was not sent to this device.")
			writeFooter(logFile, res, runStart)
			return res
		}
	}

	res.Success = true
	writeFooter(logFile, res, runStart)
	return res
}

// writeHeader prints the session's "who/what/when" so a log file is
// self-describing even opened on its own, without the CSV report next to it.
func writeHeader(w io.Writer, dev excelsheet.Device, cfg Config, lineCount int) {
	fmt.Fprintf(w, "SmartConfigure session log\n")
	fmt.Fprintf(w, "Device:        %s\n", dev.IP)
	fmt.Fprintf(w, "User:          %s\n", dev.Username)
	fmt.Fprintf(w, "Port:          %d\n", portInt(cfg.Port))
	fmt.Fprintf(w, "Template size: %d line(s)\n", lineCount)
	fmt.Fprintf(w, "Started:       %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "%s\n\n", separator)
}

// writeSection writes one clearly delimited block of the conversation: a
// title line (what we sent, or what stage we're at) followed by everything
// the device said back, so scrolling through the file reads like a
// transcript instead of an undifferentiated wall of text.
func writeSection(w io.Writer, title, body string) {
	fmt.Fprintf(w, "%s\n%s  %s\n%s\n", separator, time.Now().Format("15:04:05.000"), title, separator)
	body = strings.TrimRight(body, "\n")
	if body == "" {
		body = "(no output)"
	}
	fmt.Fprintf(w, "%s\n\n", body)
}

// writeFooter prints the final outcome of the whole run against this device.
func writeFooter(w io.Writer, res report.Result, runStart time.Time) {
	fmt.Fprintf(w, "%s\n", separator)
	if res.Success {
		fmt.Fprintf(w, "RESULT: SUCCESS\n")
	} else {
		fmt.Fprintf(w, "RESULT: FAILED — %s\n", res.Error)
	}
	fmt.Fprintf(w, "Finished:  %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "Duration:  %s\n", time.Since(runStart).Round(time.Millisecond))
	fmt.Fprintf(w, "%s\n", separator)
}

func portString(p int) string {
	return fmt.Sprintf("%d", portInt(p))
}

func portInt(p int) int {
	if p <= 0 {
		return 22
	}
	return p
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

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b`)

// cleanOutput turns a raw terminal capture into something a human can read
// top to bottom in a text editor: it normalizes line endings (devices mix
// bare \r, \r\n and \n depending on the command), strips VT100/ANSI control
// sequences some CLIs emit for pagination or color, and drops other
// non-printable bytes. Nothing from the actual conversation is removed —
// only terminal plumbing that has no meaning outside of a live terminal.
func cleanOutput(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = ansiPattern.ReplaceAllString(raw, "")

	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if r == '\n' || r == '\t' || r >= 0x20 {
			b.WriteRune(r)
		}
	}

	// Collapse runs of 3+ blank lines (common right after login banners)
	// down to a single blank line, purely for readability.
	cleaned := b.String()
	for strings.Contains(cleaned, "\n\n\n") {
		cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(cleaned)
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
		if strings.Contains(output, m) {
			return true
		}
	}
	return false
}
