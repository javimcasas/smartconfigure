package excelsheet

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Device represents one row of the input Excel file: the target's
// management IP, SSH credentials, and the values for every {VARIABLE}
// referenced in the command template.
type Device struct {
	IP       string
	Username string
	Password string
	Vars     map[string]string
}

const (
	colIP       = 0
	colUsername = 1
	colPassword = 2
)

// Load reads the first sheet of an .xlsx file. The first row must be a
// header: "IP Equipo", "Usuario", "Contraseña", followed by one column per
// template variable, named exactly like the variable (case-insensitive).
// requiredVars is the set of {VARIABLE} names found in the template; Load
// returns an error if any of them has no matching column, so a typo is
// caught before any device is touched.
func Load(path string, requiredVars []string) ([]Device, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("the Excel file has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("the Excel file needs a header row and at least one device row")
	}

	header := rows[0]
	if len(header) < 3 {
		return nil, fmt.Errorf("expected at least 3 columns (IP Equipo, Usuario, Contraseña)")
	}

	varCol := map[string]int{}
	for i := 3; i < len(header); i++ {
		name := strings.ToUpper(strings.TrimSpace(header[i]))
		if name != "" {
			varCol[name] = i
		}
	}

	var missing []string
	for _, v := range requiredVars {
		if _, ok := varCol[strings.ToUpper(v)]; !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("the Excel file is missing a column for these template variables: %s", strings.Join(missing, ", "))
	}

	var devices []Device
	for r, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		dev := Device{Vars: map[string]string{}}
		dev.IP = cell(row, colIP)
		dev.Username = cell(row, colUsername)
		dev.Password = cell(row, colPassword)

		if dev.IP == "" {
			return nil, fmt.Errorf("row %d: missing IP Equipo", r+2)
		}

		for _, v := range requiredVars {
			idx := varCol[strings.ToUpper(v)]
			dev.Vars[v] = cell(row, idx)
		}
		devices = append(devices, dev)
	}

	return devices, nil
}

func cell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
