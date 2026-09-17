package template

import "testing"

func TestRenderSubstitutesKnownValues(t *testing.T) {
	got := Render("ip address {MGMT_IP} {SUBNET_MASK}", map[string]string{"MGMT_IP": "10.0.0.1", "SUBNET_MASK": "255.255.255.0"})
	if got != "ip address 10.0.0.1 255.255.255.0" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderLeavesMissingAndEmptyValuesUnresolved(t *testing.T) {
	cases := map[string]map[string]string{
		"missing key": {},
		"empty cell":  {"SUBNET_MASK": ""},
	}
	for name, values := range cases {
		got := Render("ip address 10.0.0.1 {SUBNET_MASK}", values)
		if got != "ip address 10.0.0.1 {SUBNET_MASK}" {
			t.Errorf("%s: got %q, want the placeholder kept", name, got)
		}
		if !HasUnresolvedPlaceholder(got) {
			t.Errorf("%s: HasUnresolvedPlaceholder should be true", name)
		}
	}
}
