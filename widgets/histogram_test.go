package widgets

import (
	"testing"
)

func TestHistogram_SetBins(t *testing.T) {
	h, err := NewHistogram()
	if err != nil {
		t.Fatalf("failed to create histogram: %v", err)
	}
	bins := []int{1, 2, 3, 4, 5}
	labels := []string{"a", "b", "c", "d", "e"}
	err = h.SetBins(bins, 0, 5, labels, 2)
	if err != nil {
		t.Errorf("SetBins failed: %v", err)
	}
	if len(h.bins) != 5 {
		t.Errorf("expected 5 bins, got %d", len(h.bins))
	}
	if h.alertBin != 2 {
		t.Errorf("expected alertBin 2, got %d", h.alertBin)
	}
}
