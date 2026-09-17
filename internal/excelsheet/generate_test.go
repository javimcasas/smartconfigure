package excelsheet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestGenerateWritesHeaderThatLoadAccepts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "devices.xlsx")
	vars := []string{"SWITCH_NAME", "MGMT_IP"}
	if err := Generate(path, vars); err != nil {
		t.Fatal(err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Devices" || got[1] != "Instructions" {
		t.Fatalf("sheets = %v, want [Devices Instructions]", got)
	}
	rows, err := f.GetRows("Devices")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"IP Address", "Username", "Password", "SWITCH_NAME", "MGMT_IP"}
	if len(rows) != 1 || len(rows[0]) != len(want) {
		t.Fatalf("header rows = %v, want just %v", rows, want)
	}
	for i := range want {
		if rows[0][i] != want[i] {
			t.Errorf("col %d = %q, want %q", i, rows[0][i], want[i])
		}
	}

	// An empty generated sheet must be rejected by Load with the "needs at
	// least one device row" message, not with a column error.
	if _, err := Load(path, vars); err == nil {
		t.Error("Load should refuse a sheet with no device rows")
	}

	// Fill a row through excelize and make sure Load reads it back.
	_ = f.SetSheetRow("Devices", "A2", &[]interface{}{"10.0.0.1", "admin", "pw", "SW-01", "10.0.0.1"})
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	devices, err := Load(path, vars)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].IP != "10.0.0.1" || devices[0].Vars["MGMT_IP"] != "10.0.0.1" {
		t.Errorf("Load returned %+v", devices)
	}
}

func TestDefaultExcelPathNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "template.txt")
	if got := DefaultExcelPath(tmpl); got != filepath.Join(dir, "devices.xlsx") {
		t.Fatalf("first path = %s", got)
	}
	_ = os.WriteFile(filepath.Join(dir, "devices.xlsx"), nil, 0o644)
	if got := DefaultExcelPath(tmpl); got != filepath.Join(dir, "devices-2.xlsx") {
		t.Fatalf("second path = %s", got)
	}
	_ = os.WriteFile(filepath.Join(dir, "devices-2.xlsx"), nil, 0o644)
	if got := DefaultExcelPath(tmpl); got != filepath.Join(dir, "devices-3.xlsx") {
		t.Fatalf("third path = %s", got)
	}
}
