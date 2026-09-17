package securecrt

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
)

// go test ./internal/securecrt -update  rewrites the golden files.
var update = flag.Bool("update", false, "rewrite golden files")

// fixtureBatch mirrors example/template.txt (first lines) rendered for two
// devices, with a deliberately nasty password: quotes, ampersand,
// backslash, tab and a non-ASCII rune, to pin the escaping of both
// flavours.
func fixtureBatch() Batch {
	tmpl := []string{
		"system-view",
		"sysname {SWITCH_NAME}",
		"commit",
		"aaa",
		" local-user sshadmin password irreversible-cipher {SSH_PASSWORD}",
		"commit",
		"ip route-static vpn-instance _management_vpn_ 0.0.0.0 0.0.0.0 {GATEWAY_IP}",
		"interface MEth0/0/0",
		" ip address {MGMT_IP} {SUBNET_MASK}",
		"commit",
		"save",
	}
	devices := []excelsheet.Device{
		{IP: "10.10.1.21", Username: "admin", Password: `p"a&s\s	wörd`, Vars: map[string]string{
			"SWITCH_NAME": "SW-CORE-01", "SSH_PASSWORD": "Ssh#2026", "GATEWAY_IP": "10.10.1.1",
			"MGMT_IP": "10.10.1.21", "SUBNET_MASK": "255.255.255.0",
		}},
		{IP: "10.10.1.22", Username: "admin", Password: "plain", Vars: map[string]string{
			"SWITCH_NAME": "SW-CORE-02", "SSH_PASSWORD": "Ssh#2026", "GATEWAY_IP": "10.10.1.1",
			"MGMT_IP": "10.10.1.22", "SUBNET_MASK": "255.255.255.0",
		}},
	}
	rendered, err := Build(devices, tmpl)
	if err != nil {
		panic(err)
	}
	return Batch{
		Version:      "v0.2.0",
		GeneratedAt:  time.Date(2026, 9, 17, 10, 30, 0, 0, time.UTC),
		TemplatePath: `C:\work\template.txt`,
		ExcelPath:    `C:\work\equipos.xlsx`,
		Port:         22,
		IdleTimeout:  800 * time.Millisecond,
		LineTimeout:  15 * time.Second,
		Devices:      rendered,
	}
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run with -update): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden file (run with -update after reviewing):\n%s", name, diffHint(got, want))
	}
}

func diffHint(got, want []byte) string {
	g := strings.Split(string(got), "\r\n")
	w := strings.Split(string(want), "\r\n")
	for i := 0; i < len(g) && i < len(w); i++ {
		if g[i] != w[i] {
			return "first difference at line " + strconv.Itoa(i+1) + ":\n  got:  " + g[i] + "\n  want: " + w[i]
		}
	}
	return "line count differs: got " + strconv.Itoa(len(g)) + ", want " + strconv.Itoa(len(w))
}

func TestRenderVBScriptGolden(t *testing.T) {
	checkGolden(t, "batch.vbs.golden", RenderVBScript(fixtureBatch()))
}

func TestRenderPythonGolden(t *testing.T) {
	checkGolden(t, "batch.py.golden", RenderPython(fixtureBatch()))
}

func TestScriptsAreCRLFAndASCII(t *testing.T) {
	for name, out := range map[string][]byte{"vbs": RenderVBScript(fixtureBatch()), "py": RenderPython(fixtureBatch())} {
		if bytes.Contains(bytes.ReplaceAll(out, []byte("\r\n"), nil), []byte("\n")) {
			t.Errorf("%s: found a bare LF", name)
		}
		for i, b := range out {
			if b > 0x7e || (b < 0x20 && b != '\r' && b != '\n') {
				t.Errorf("%s: non-ASCII byte 0x%02x at offset %d", name, b, i)
				break
			}
		}
	}
}

func TestVBStringEscaping(t *testing.T) {
	cases := map[string]string{
		`plain`:    `"plain"`,
		`say "hi"`: `"say ""hi"""`,
		`wörd`:     `"w" & ChrW(246) & "rd"`,
		`ö`:        `"" & ChrW(246)`,
		`a	b`:      `"a" & ChrW(9) & "b"`,
		`öö`:       `"" & ChrW(246) & ChrW(246)`,
		``:         `""`,
	}
	for in, want := range cases {
		if got := vbString(in); got != want {
			t.Errorf("vbString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestPyStringEscaping(t *testing.T) {
	cases := map[string]string{
		`plain`:    `"plain"`,
		`say "hi"`: `"say \"hi\""`,
		`back\sl`:  `"back\\sl"`,
		`wörd`:     `"w\u00f6rd"`,
		"a\tb":     `"a\tb"`,
	}
	for in, want := range cases {
		if got := pyString(in); got != want {
			t.Errorf("pyString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestBuildRejectsUnresolvedPlaceholder(t *testing.T) {
	devices := []excelsheet.Device{{IP: "10.0.0.1", Username: "u", Password: "p", Vars: map[string]string{"A": "1"}}}
	_, err := Build(devices, []string{"set {A}", "set {B}"})
	if err == nil {
		t.Fatal("expected an error for the unresolved {B}")
	}
	if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "{B}") {
		t.Errorf("error should name the line and the placeholder, got: %v", err)
	}
}

func TestWriteProducesBothFiles(t *testing.T) {
	dir := t.TempDir()
	paths, err := Write(fixtureBatch(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s not written: %v", p, err)
		}
	}
	if _, err := Write(Batch{}, dir); err == nil {
		t.Error("expected an error for an empty batch")
	}
}

func TestSecondsCeil(t *testing.T) {
	cases := map[time.Duration]int{0: 1, 500 * time.Millisecond: 1, time.Second: 1, 1500 * time.Millisecond: 2, 15 * time.Second: 15}
	for d, want := range cases {
		if got := secondsCeil(d); got != want {
			t.Errorf("secondsCeil(%v) = %d, want %d", d, got, want)
		}
	}
}
