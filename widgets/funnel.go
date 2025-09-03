package widgets

import (
	"errors"
	"fmt"
	"image"
	"sync"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/private/canvas"
	"github.com/mum4k/termdash/private/canvas/braille"
	"github.com/mum4k/termdash/private/draw"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
)

// Funnel displays data as a funnel chart.
// Each value represents a segment in the funnel, with the top being the widest.
type Funnel struct {
	mu sync.Mutex

	// values holds the data for each segment.
	values []int
	// colors holds the color for each segment.
	colors []cell.Color
	// total is the sum of all values.
	total int
}

// NewFunnel returns a new Funnel widget.
func NewFunnel() (*Funnel, error) {
	return &Funnel{}, nil
}

// Values sets the data for the funnel chart.
// The values must be non-negative. A color must be provided for each value.
// If not enough colors are provided, they will be reused.
func (f *Funnel) Values(values []int, colors []cell.Color) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(values) == 0 {
		return errors.New("values cannot be empty")
	}
	if len(colors) == 0 {
		return errors.New("colors cannot be empty")
	}

	f.values = values
	f.colors = colors
	f.total = 0
	for _, v := range values {
		if v < 0 {
			return errors.New("all values must be non-negative")
		}
		f.total += v
	}

	return nil
}

// Draw draws the Funnel widget onto the canvas.
func (f *Funnel) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.total <= 0 {
		return nil
	}

	bc, err := braille.New(cvs.Area())
	if err != nil {
		return fmt.Errorf("braille.New => %v", err)
	}

	ar := cvs.Area()
	// Funnel will be drawn in the center of the canvas
	centerX := ar.Min.X*braille.ColMult + ar.Dx()*braille.ColMult/2
	funnelHeight := ar.Dy() * braille.RowMult - 2 // Leave some padding
	topWidth := ar.Dx() * braille.ColMult - 2
	
	// A small value for the bottom width to ensure a pointed funnel shape.
	bottomWidth := 5

	currentY := ar.Min.Y*braille.RowMult + 1
	cumulativeValue := 0

	for i, value := range f.values {
		// Calculate the height of the current segment based on its proportion of the total.
		segmentHeight := int(float64(value) / float64(f.total) * float64(funnelHeight))
		if segmentHeight < 1 {
			// Ensure a minimum height for very small values.
			segmentHeight = 1
		}

		// Calculate the width of the top and bottom of the current segment.
		// The width tapers linearly from topWidth to bottomWidth across the funnel height.
		topProportion := float64(cumulativeValue) / float64(f.total)
		bottomProportion := float64(cumulativeValue+value) / float64(f.total)

		currentTopWidth := int(float64(topWidth) - (float64(topWidth-bottomWidth) * topProportion))
		currentBottomWidth := int(float64(topWidth) - (float64(topWidth-bottomWidth) * bottomProportion))

		color := f.colors[i%len(f.colors)]

		// Fill the trapezoid of the segment with horizontal lines.
		for y := 0; y < segmentHeight; y++ {
			// The width of the line at the current y-coordinate within the segment.
			lineProportion := float64(y) / float64(segmentHeight)
			lineWidth := int(float64(currentTopWidth) - (float64(currentTopWidth-currentBottomWidth) * lineProportion))
			
			lineStart := image.Point{X: centerX - lineWidth/2, Y: currentY + y}
			lineEnd := image.Point{X: centerX + lineWidth/2, Y: currentY + y}
			
			if err := draw.BrailleLine(bc, lineStart, lineEnd, draw.BrailleLineCellOpts(cell.FgColor(color))); err != nil {
				return fmt.Errorf("failed to draw funnel segment line: %v", err)
			}
		}

		currentY += segmentHeight
		cumulativeValue += value
	}
	
	if err := bc.CopyTo(cvs); err != nil {
		return err
	}

	return nil
}

// Keyboard input isn't supported on the Funnel widget.
func (*Funnel) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return errors.New("the Funnel widget doesn't support keyboard events")
}

// Mouse input isn't supported on the Funnel widget.
func (*Funnel) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	return errors.New("the Funnel widget doesn't support mouse events")
}

// Options implements widgetapi.Widget.Options.
func (f *Funnel) Options() widgetapi.Options {
	return widgetapi.Options{
		Ratio:        image.Point{braille.RowMult, braille.ColMult},
		MinimumSize:  image.Point{5, 5},
		WantKeyboard: widgetapi.KeyScopeNone,
		WantMouse:    widgetapi.MouseScopeNone,
	}
}
