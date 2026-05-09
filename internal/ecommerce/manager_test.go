package ecommerce

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractAllOrderNumbers(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "prefixed order keyword",
			in:   "Hi, my order 100123456 hasn't arrived",
			want: []string{"100123456"},
		},
		{
			name: "hash prefix",
			in:   "Following up on #100654321 please",
			want: []string{"100654321"},
		},
		{
			name: "standalone Magento-style id",
			in:   "Looking for 100888777 status update",
			want: []string{"100888777"},
		},
		{
			name: "mix of prefixed and standalone (deduped)",
			in:   "order: 100123456 and also 100222333 — for #100123456",
			want: []string{"100123456", "100222333"},
		},
		{
			name: "no order numbers",
			in:   "Just a generic question about returns",
			want: nil,
		},
		{
			name: "too-short numbers ignored",
			in:   "phone 555-1212 reference 12345",
			want: nil,
		},
		{
			name: "case insensitive prefix",
			in:   "ORDER NUMBER: 100999888",
			want: []string{"100999888"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAllOrderNumbers(tc.in)
			// extractAllOrderNumbers can return duplicates across the two regex
			// passes; the manager dedupes downstream. For the test we compare
			// unique sorted sets.
			gotUnique := dedupSorted(got)
			wantUnique := dedupSorted(tc.want)
			if !reflect.DeepEqual(gotUnique, wantUnique) {
				t.Fatalf("extractAllOrderNumbers(%q) = %v, want %v", tc.in, gotUnique, wantUnique)
			}
		})
	}
}

func dedupSorted(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func TestTrackingURL(t *testing.T) {
	cases := []struct {
		carrier string
		track   string
		want    string
	}{
		{"Australia Post", "ABC123", "https://auspost.com.au/mypost/track/details/ABC123"},
		{"AusPost eParcel", "X1", "https://auspost.com.au/mypost/track/details/X1"},
		{"Couriers Please", "CP9", "https://www.couriersplease.com.au/tools-track/no/CP9"},
		{"Toll TGE", "TGE7", "https://www.myteamge.com/?externalSearchQuery=TGE7"},
		{"Team Global Express", "TGE8", "https://www.myteamge.com/?externalSearchQuery=TGE8"},
		{"FedEx", "FX1", ""},
		{"", "X", ""},
	}
	for _, tc := range cases {
		t.Run(tc.carrier, func(t *testing.T) {
			got := trackingURL(tc.carrier, tc.track)
			if got != tc.want {
				t.Fatalf("trackingURL(%q, %q) = %q, want %q", tc.carrier, tc.track, got, tc.want)
			}
		})
	}
}
