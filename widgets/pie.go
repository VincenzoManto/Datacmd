package widgets

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/private/canvas"
	"github.com/mum4k/termdash/private/canvas/braille"
	"github.com/mum4k/termdash/private/draw"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
)

// probably it will be substituted with my termdash/pie once approved by murr (please approve my PR)

// PieChart displays data as a pie chart.
// Each value is a proportion of the total sum.
type PieChart struct {
	mu sync.Mutex

	// values holds the data for each slice.
	values []int
	// colors holds the color for each slice.
	colors []cell.Color
	// total is the sum of all values.
	total int
}

// pieChartOption is used to provide options to the piechart widget.
type pieChartOption interface {
	set(*pieChartOptions)
}

// pieChartOptions stores the provided options.
type pieChartOptions struct{}

// NewPieChart returns a new PieChart widget.
func NewPieChart() (*PieChart, error) {
	return &PieChart{}, nil
}

// Values sets the data for the pie chart.
// The values must be non-negative. A color must be provided for each value.
// If not enough colors are provided, they will be reused.
func (p *PieChart) Values(values []int, colors []cell.Color) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(values) == 0 {
		return errors.New("values cannot be empty")
	}
	if len(colors) == 0 {
		return errors.New("colors cannot be empty")
	}

	p.values = values
	p.colors = colors
	p.total = 0
	for _, v := range values {
		if v < 0 {
			return errors.New("all values must be non-negative")
		}
		p.total += v
	}

	return nil
}

// pieChartMidAndRadii returns the center point and horizontal and vertical radii.
func pieChartMidAndRadii(ar image.Rectangle) (image.Point, int, int) {
	width := ar.Dx() * braille.ColMult
	height := ar.Dy() * braille.RowMult

	radiusX := width/2 - 2
	radiusY := height/2 - 2
	if radiusX < 1 {
		radiusX = 1
	}
	if radiusY < 1 {
		radiusY = 1
	}
	mid := image.Point{
		X: ar.Min.X*braille.ColMult + width/2,
		Y: ar.Min.Y*braille.RowMult + height/2,
	}
	return mid, radiusX, radiusY
}

// Draw draws the PieChart widget onto the canvas.
func (p *PieChart) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.total <= 0 {
		return nil
	}

	bc, err := braille.New(cvs.Area())
	if err != nil {
		return fmt.Errorf("braille.New => %v", err)
	}

	mid, radiusX, radiusY := pieChartMidAndRadii(cvs.Area())

	innerRadiusX := int(float64(radiusX) * 0.6)
	innerRadiusY := int(float64(radiusY) * 0.6)

	currentAngle := 0.0
	for i, value := range p.values {
		endAngle := currentAngle + float64(value)/float64(p.total)*2*math.Pi
		color := p.colors[i%len(p.colors)]

		for angle := currentAngle; angle < endAngle; angle += 0.01 { 
			startX := mid.X + int(float64(innerRadiusX)*math.Cos(angle))
			startY := mid.Y + int(float64(innerRadiusY)*math.Sin(angle))

			endX := mid.X + int(float64(radiusX)*math.Cos(angle))
			endY := mid.Y + int(float64(radiusY)*math.Sin(angle))

			startPoint := image.Point{X: startX, Y: startY}
			endPoint := image.Point{X: endX, Y: endY}

			if err := draw.BrailleLine(bc, startPoint, endPoint, draw.BrailleLineCellOpts(cell.FgColor(color))); err != nil {
				return fmt.Errorf("failed to draw donut slice line: %v", err)
			}
		}

		currentAngle = endAngle
	}

	if err := bc.CopyTo(cvs); err != nil {
		return err
	}

	return nil
}

// Keyboard input isn't supported on the PieChart widget.
func (*PieChart) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return errors.New("the PieChart widget doesn't support keyboard events")
}

// Mouse input isn't supported on the PieChart widget.
func (*PieChart) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	return errors.New("the PieChart widget doesn't support mouse events")
}

// Options implements widgetapi.Widget.Options.
func (p *PieChart) Options() widgetapi.Options {
	return widgetapi.Options{
		Ratio:        image.Point{braille.RowMult, braille.ColMult},
		MinimumSize:  image.Point{5, 5},
		WantKeyboard: widgetapi.KeyScopeNone,
		WantMouse:    widgetapi.MouseScopeNone,
	}
}
