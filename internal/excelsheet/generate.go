package excelsheet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Fixed columns every devices workbook starts with. Load reads them by
// position, so the header text is for the person filling the sheet.
var fixedHeader = []string{"IP Address", "Username", "Password"}

const (
	devicesSheet      = "Devices"
	instructionsSheet = "Instructions"
	defaultExcelName  = "devices.xlsx"
)

// Generate writes an empty devices workbook for a template: the three
// fixed columns plus one column per template variable, a bold frozen
// header, sensible widths, and an Instructions sheet. The user fills one
// row per device and feeds the file back to Run / Export. Nothing is
// pre-filled on purpose: a sample row with a made-up IP is a row that
// could end up in a real batch.
func Generate(path string, vars []string) error {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", devicesSheet); err != nil {
		return err
	}
	header := append(append([]string{}, fixedHeader...), vars...)
	for i, name := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(devicesSheet, cell, name); err != nil {
			return err
		}
	}
	lastCol, _ := excelize.ColumnNumberToName(len(header))
	bold, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#DBEAFE"}},
	})
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(devicesSheet, "A1", lastCol+"1", bold); err != nil {
		return err
	}
	if err := f.SetColWidth(devicesSheet, "A", lastCol, 22); err != nil {
		return err
	}
	if err := f.SetPanes(devicesSheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}

	if _, err := f.NewSheet(instructionsSheet); err != nil {
		return err
	}
	lines := []string{
		"SmartConfigure — devices workbook",
		"",
		"Fill the Devices sheet with one row per device:",
		"  A  IP Address   management IP the tool connects to over SSH",
		"  B  Username     SSH user",
		"  C  Password     SSH password (this file is sensitive — keep it private)",
		fmt.Sprintf("  D… one column per template variable: %s", strings.Join(vars, ", ")),
		"",
		"Rules:",
		"  - Every variable column needs a value in every row; an empty cell marks that device FAILED (dry-run catches it).",
		"  - Column names are matched to {VARIABLES} case-insensitively; do not rename them.",
		"  - Only the first sheet (Devices) is read; the first row must stay the header.",
		"  - Empty rows are ignored.",
	}
	for i, l := range lines {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetCellValue(instructionsSheet, cell, l); err != nil {
			return err
		}
	}
	if err := f.SetColWidth(instructionsSheet, "A", "A", 110); err != nil {
		return err
	}
	f.SetActiveSheet(0)

	return f.SaveAs(path)
}

// DefaultExcelPath returns a devices.xlsx path next to the template that
// does not exist yet (devices.xlsx, devices-2.xlsx, …), so generating
// twice never overwrites a sheet someone already filled in.
func DefaultExcelPath(templatePath string) string {
	dir := filepath.Dir(templatePath)
	candidate := filepath.Join(dir, defaultExcelName)
	for i := 2; fileExists(candidate); i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("devices-%d.xlsx", i))
	}
	return candidate
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
