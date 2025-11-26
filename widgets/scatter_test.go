package widgets

import (
	"testing"
)

func TestScatterPlot_SetPoints(t *testing.T) {
	sp, err := NewScatterPlot()
	if err != nil {
		t.Fatalf("failed to create scatter plot: %v", err)
	}
	points := []ScatterPoint{{X: 1, Y: 2}, {X: 2, Y: 3}, {X: 3, Y: 4}}
	err = sp.SetPoints(points, "x", "y")
	if err != nil {
		t.Errorf("SetPoints failed: %v", err)
	}
	if len(sp.points) != 3 {
		t.Errorf("expected 3 points, got %d", len(sp.points))
	}
}
