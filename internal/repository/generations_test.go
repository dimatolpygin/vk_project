package repository

import "testing"

func TestPickChargedBalanceKindPrefersPaidBalance(t *testing.T) {
	tests := []struct {
		name     string
		paidGens int
		freeGens int
		wantKind string
		wantOK   bool
	}{
		{name: "paid first", paidGens: 3, freeGens: 2, wantKind: BalanceKindPaid, wantOK: true},
		{name: "free fallback", paidGens: 0, freeGens: 2, wantKind: BalanceKindFree, wantOK: true},
		{name: "no balance", paidGens: 0, freeGens: 0, wantKind: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotOK := pickChargedBalanceKind(tt.paidGens, tt.freeGens)
			if gotKind != tt.wantKind || gotOK != tt.wantOK {
				t.Fatalf("pickChargedBalanceKind(%d, %d) = (%q, %v), want (%q, %v)", tt.paidGens, tt.freeGens, gotKind, gotOK, tt.wantKind, tt.wantOK)
			}
		})
	}
}

func TestLegacyBalanceKindFromStatus(t *testing.T) {
	if got := legacyBalanceKindFromStatus("paid"); got != BalanceKindPaid {
		t.Fatalf("expected paid status to map to %q, got %q", BalanceKindPaid, got)
	}
	if got := legacyBalanceKindFromStatus("free"); got != BalanceKindFree {
		t.Fatalf("expected non-paid status to map to %q, got %q", BalanceKindFree, got)
	}
}
