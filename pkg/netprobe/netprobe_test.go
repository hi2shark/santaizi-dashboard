package netprobe

import "testing"

func TestParsePorts(t *testing.T) {
	ports := ParsePorts("22, 443,22,0,abc")
	if len(ports) != 2 || ports[0] != 22 || ports[1] != 443 {
		t.Fatalf("%v", ports)
	}
}

func TestFormatHostPrefersOverrideThenIPv4(t *testing.T) {
	if got := FormatHost("1.1.1.1", "::1", "origin.example"); got != "origin.example" {
		t.Fatal(got)
	}
	if got := FormatHost("1.1.1.1", "::1", ""); got != "1.1.1.1" {
		t.Fatal(got)
	}
}

func TestDisplayErrorWhenAnyTCPSucceeds(t *testing.T) {
	if err := DisplayError(ICMPResult{OK: false, Error: "timeout"}, []TCPResult{{Port: 443, OK: true}}); err != "" {
		t.Fatal(err)
	}
	if err := DisplayError(ICMPResult{OK: false, Error: "timeout"}, []TCPResult{{Port: 22, Error: "refused"}}); err != "timeout" {
		t.Fatal(err)
	}
}
