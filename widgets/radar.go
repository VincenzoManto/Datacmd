package widgets

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/private/canvas"
	"github.com/mum4k/termdash/private/canvas/braille"
	"github.com/mum4k/termdash/private/draw"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
)

// RadarOption is used to provide options to the radar widget.
type RadarOption interface {
	set(*radarOptions)
}

// radarOptions stores the provided options.
type radarOptions struct {
	axisCellOpts []cell.Option
	dataCellOpts []cell.Option
}

// newRadarOptions returns a new radarOptions struct with default values.
func newRadarOptions() *radarOptions {
	return &radarOptions{
		axisCellOpts: []cell.Option{cell.FgColor(cell.ColorNumber(240))},
		dataCellOpts: []cell.Option{cell.FgColor(cell.ColorNumber(42))},
	}
}

// withAxisColor is a private type that implements the RadarOption interface.
type withAxisColor struct {
	c int
}

func (w *withAxisColor) set(opts *radarOptions) {
	opts.axisCellOpts = []cell.Option{cell.FgColor(cell.ColorNumber(w.c))}
}

// WithAxisColor sets the color for the axes of the radar chart using a 256-color number.
func WithAxisColor(c int) RadarOption {
	return &withAxisColor{c: c}
}

// withDataColor is a private type that implements the RadarOption interface.
type withDataColor struct {
	c int
}

func (w *withDataColor) set(opts *radarOptions) {
	opts.dataCellOpts = []cell.Option{cell.FgColor(cell.ColorNumber(w.c))}
}

// WithDataColor sets the color for the data polygon using a 256-color number.
func WithDataColor(c int) RadarOption {
	return &withDataColor{c: c}
}

// Internal validation function.
func (o *radarOptions) validate() error {
	return nil
}

// midAndRadii returns the center point and horizontal and vertical radii of the
// largest ellipse that can be drawn on the braille canvas.
func midAndRadii(ar image.Rectangle) (image.Point, int, int) {
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


type Values struct {
	// A map to store the value for each axis, using a string label as the key.
	Data map[string]float64
	// The maximum possible value.
	Max float64
}

// Radar displays multivariate data on a radar chart.
type Radar struct {
	mu sync.Mutex

	// The data to be drawn.
	values *Values

	// opts are the provided options.
	opts *radarOptions
}

// NewRadar returns a new Radar chart.
func NewRadar(opts ...RadarOption) (*Radar, error) {
	opt := newRadarOptions()
	for _, o := range opts {
		o.set(opt)
	}
	if err := opt.validate(); err != nil {
		return nil, err
	}
	return &Radar{
		opts: opt,
	}, nil
}

// SetValues sets the data to be displayed on the chart.
func (r *Radar) SetValues(vals *Values, opts ...RadarOption) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if vals == nil || len(vals.Data) < 3 {
		return fmt.Errorf("values cannot be nil or empty, and a radar chart requires at least 3 data points")
	}
	if vals.Max <= 0 {
		return fmt.Errorf("maximum value must be greater than zero")
	}
	for _, v := range vals.Data {
		if v < 0 || v > vals.Max {
			return fmt.Errorf("value %f is outside the valid range [0, %f]", v, vals.Max)
		}
	}

	for _, opt := range opts {
		opt.set(r.opts)
	}
	if err := r.opts.validate(); err != nil {
		return err
	}

	r.values = vals
	return nil
}

// Draw draws the Radar widget onto the canvas.
// Implements widgetapi.Widget.Draw.
func (r *Radar) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.values == nil || len(r.values.Data) < 3 {
		return nil
	}

	bc, err := braille.New(cvs.Area())
	if err != nil {
		return fmt.Errorf("braille.New => %v", err)
	}
	
	mid, radiusX, radiusY := midAndRadii(cvs.Area())

	if err := draw.BrailleCircle(bc, mid, radiusX,
		draw.BrailleCircleCellOpts(r.opts.axisCellOpts...)); err != nil {
		return fmt.Errorf("failed to draw external circle: %v", err)
	}

	numAxes := len(r.values.Data)
	angleStep := 2 * math.Pi / float64(numAxes)

	var dataPoints []image.Point

	keys := make([]string, 0, len(r.values.Data))
	for k := range r.values.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, label := range keys {
		value := r.values.Data[label]
		angle := float64(i)*angleStep - math.Pi/2

		endX := mid.X + int(float64(radiusX)*math.Cos(angle))
		endY := mid.Y + int(float64(radiusY)*math.Sin(angle))

		if err := draw.BrailleLine(bc, mid, image.Point{X: endX, Y: endY}, draw.BrailleLineCellOpts(r.opts.axisCellOpts...)); err != nil {
			return fmt.Errorf("failed to draw axis: %v", err)
		}

		valRx := float64(value) / r.values.Max * float64(radiusX)
		valRy := float64(value) / r.values.Max * float64(radiusY)
		pointX := mid.X + int(valRx*math.Cos(angle))
		pointY := mid.Y + int(valRy*math.Sin(angle))

		dataPoints = append(dataPoints, image.Point{X: pointX, Y: pointY})
	}

	for j := 0; j < len(dataPoints)-1; j++ {
		if err := draw.BrailleLine(bc, dataPoints[j], dataPoints[j+1], draw.BrailleLineCellOpts(r.opts.dataCellOpts...)); err != nil {
			return fmt.Errorf("failed to draw data line: %v", err)
		}
	}
	if err := draw.BrailleLine(bc, dataPoints[len(dataPoints)-1], dataPoints[0], draw.BrailleLineCellOpts(r.opts.dataCellOpts...)); err != nil {
		return fmt.Errorf("failed to close data polygon: %v", err)
	}

	if err := bc.CopyTo(cvs); err != nil {
		return err
	}

	return nil
}

// Keyboard input isn't supported on the Radar widget.
func (*Radar) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return errors.New("the Radar widget doesn't support keyboard events")
}

// Mouse input isn't supported on the Radar widget.
func (*Radar) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	return errors.New("the Radar widget doesn't support mouse events")
}

// minSize is the smallest area we can draw a radar chart on.
var minSize = image.Point{X: 5, Y: 5}

// Options implements widgetapi.Widget.Options.
func (r *Radar) Options() widgetapi.Options {
	return widgetapi.Options{
		Ratio:        image.Point{X: braille.RowMult, Y: braille.ColMult},
		MinimumSize:  minSize,
		WantKeyboard: widgetapi.KeyScopeNone,
		WantMouse:    widgetapi.MouseScopeNone,
	}
}
