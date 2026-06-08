package target

import (
	"reflect"
	"testing"
)

func TestExpand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		maxHosts int
		wantIDs  []string
		wantErr  bool
	}{
		{
			name: "single ipv4", args: []string{"127.0.0.1"}, maxHosts: 10,
			wantIDs: []string{"127.0.0.1"},
		},
		{
			name: "single ipv6", args: []string{"::1"}, maxHosts: 10,
			wantIDs: []string{"::1"},
		},
		{
			name: "hostname", args: []string{"example.com"}, maxHosts: 10,
			wantIDs: []string{"example.com"},
		},
		{
			name: "slash 32", args: []string{"10.0.0.5/32"}, maxHosts: 10,
			wantIDs: []string{"10.0.0.5"},
		},
		{
			name: "slash 31 keeps both", args: []string{"10.0.0.0/31"}, maxHosts: 10,
			wantIDs: []string{"10.0.0.0", "10.0.0.1"},
		},
		{
			name: "slash 30 skips network and broadcast", args: []string{"10.0.0.0/30"}, maxHosts: 10,
			wantIDs: []string{"10.0.0.1", "10.0.0.2"},
		},
		{
			name: "slash 29 yields six hosts", args: []string{"10.0.0.0/29"}, maxHosts: 10,
			wantIDs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "10.0.0.6"},
		},
		{
			name: "ipv6 /126 keeps all four", args: []string{"2001:db8::/126"}, maxHosts: 10,
			wantIDs: []string{"2001:db8::", "2001:db8::1", "2001:db8::2", "2001:db8::3"},
		},
		{
			name: "mixed inputs preserve order",
			args: []string{"1.1.1.1", "example.com", "10.0.0.0/30"}, maxHosts: 10,
			wantIDs: []string{"1.1.1.1", "example.com", "10.0.0.1", "10.0.0.2"},
		},
		{
			name: "deduplicated", args: []string{"1.1.1.1", "1.1.1.1"}, maxHosts: 10,
			wantIDs: []string{"1.1.1.1"},
		},
		{
			name: "empty arg ignored", args: []string{"", "1.1.1.1"}, maxHosts: 10,
			wantIDs: []string{"1.1.1.1"},
		},
		{
			name: "cidr host bits get masked", args: []string{"10.0.0.5/30"}, maxHosts: 10,
			wantIDs: []string{"10.0.0.5", "10.0.0.6"},
		},

		{name: "over cap single CIDR", args: []string{"10.0.0.0/24"}, maxHosts: 4, wantErr: true},
		{name: "over cap accumulated", args: []string{"10.0.0.0/29", "10.0.1.0/29"}, maxHosts: 10, wantErr: true},
		{name: "over cap from singles", args: []string{"1.1.1.1", "1.1.1.2", "1.1.1.3"}, maxHosts: 2, wantErr: true},
		{name: "invalid CIDR mask", args: []string{"10.0.0.0/99"}, maxHosts: 10, wantErr: true},
		{name: "garbage arg", args: []string{"!!!"}, maxHosts: 10, wantErr: true},
		{name: "malformed ipv4 rejected", args: []string{"273.25.17.2555"}, maxHosts: 10, wantErr: true},
		{name: "truncated ipv4 rejected", args: []string{"192.168.1"}, maxHosts: 10, wantErr: true},
		{name: "all-numeric label rejected", args: []string{"123"}, maxHosts: 10, wantErr: true},
		{
			name: "hostname starting with digit accepted",
			args: []string{"1.example.com"}, maxHosts: 10,
			wantIDs: []string{"1.example.com"},
		},
		{
			name: "bare localhost accepted",
			args: []string{"localhost"}, maxHosts: 10,
			wantIDs: []string{"localhost"},
		},
		{name: "zero maxHosts rejected", args: []string{"1.1.1.1"}, maxHosts: 0, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Expand(tc.args, tc.maxHosts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotIDs := make([]string, len(got))
			for i, x := range got {
				gotIDs[i] = x.ID
				if x.Host == "" {
					t.Errorf("target %q has empty Host", x.ID)
				}
			}
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Fatalf("ids = %v, want %v", gotIDs, tc.wantIDs)
			}
		})
	}
}
