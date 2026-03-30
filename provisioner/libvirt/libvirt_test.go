package libvirt

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	outrunner "github.com/psubocz/gha-outrunner"
)

func TestRenderDomainXML(t *testing.T) {
	tests := []struct {
		name    string
		vmName  string
		overlay string
		img     *outrunner.LibvirtImage
		network string
		check   func(t *testing.T, xmlStr string)
	}{
		{
			name:    "basic",
			vmName:  "test-runner",
			overlay: "/tmp/test-runner.qcow2",
			img:     &outrunner.LibvirtImage{CPUs: 4, MemoryMB: 8192},
			network: "default",
			check: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<name>test-runner</name>") {
					t.Error("missing VM name")
				}
				if !strings.Contains(xmlStr, "<memory unit='MiB'>8192</memory>") {
					t.Error("missing memory")
				}
				if !strings.Contains(xmlStr, "<vcpu>4</vcpu>") {
					t.Error("missing vcpu")
				}
				if !strings.Contains(xmlStr, "<source file='/tmp/test-runner.qcow2'/>") {
					t.Error("missing overlay path")
				}
				if !strings.Contains(xmlStr, "<source network='default'/>") {
					t.Error("missing network")
				}
			},
		},
		{
			name:    "high resources",
			vmName:  "beefy-vm",
			overlay: "/var/lib/libvirt/overlays/beefy-vm.qcow2",
			img:     &outrunner.LibvirtImage{CPUs: 32, MemoryMB: 65536},
			network: "br0",
			check: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<vcpu>32</vcpu>") {
					t.Error("expected 32 vcpus")
				}
				if !strings.Contains(xmlStr, "<memory unit='MiB'>65536</memory>") {
					t.Error("expected 65536 MiB memory")
				}
				if !strings.Contains(xmlStr, "<source network='br0'/>") {
					t.Error("expected br0 network")
				}
			},
		},
		{
			name:    "minimal resources",
			vmName:  "tiny",
			overlay: "/tmp/tiny.qcow2",
			img:     &outrunner.LibvirtImage{CPUs: 1, MemoryMB: 512},
			network: "default",
			check: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<vcpu>1</vcpu>") {
					t.Error("expected 1 vcpu")
				}
				if !strings.Contains(xmlStr, "<memory unit='MiB'>512</memory>") {
					t.Error("expected 512 MiB memory")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderDomainXML(tt.vmName, tt.overlay, tt.img, tt.network)
			if err != nil {
				t.Fatalf("renderDomainXML: %v", err)
			}

			// Verify it's valid XML
			var parsed struct{}
			if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
				t.Errorf("generated XML is not valid: %v\n%s", err, result)
			}

			// Verify expected structure
			if !strings.Contains(result, "<domain type='kvm'>") {
				t.Error("missing domain type")
			}
			if !strings.Contains(result, "org.qemu.guest_agent.0") {
				t.Error("missing guest agent channel")
			}

			tt.check(t, result)
		})
	}
}

func TestGuestExecResultParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantPID int
		wantErr bool
	}{
		{
			name:    "valid response",
			json:    `{"return":{"pid":1234}}`,
			wantPID: 1234,
		},
		{
			name:    "pid zero",
			json:    `{"return":{"pid":0}}`,
			wantPID: 0,
		},
		{
			name:    "malformed json",
			json:    `{invalid`,
			wantErr: true,
		},
		{
			name:    "missing return field",
			json:    `{"other":"data"}`,
			wantPID: 0, // zero value, not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result guestExecResult
			err := json.Unmarshal([]byte(tt.json), &result)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Return.PID != tt.wantPID {
				t.Errorf("expected PID %d, got %d", tt.wantPID, result.Return.PID)
			}
		})
	}
}
