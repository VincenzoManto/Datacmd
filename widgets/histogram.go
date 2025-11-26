package widgets

import (
	"errors"
	"image"
	"math"
	"sync"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/private/canvas"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
)

// Histogram displays a histogram of a numeric column.
type Histogram struct {
	mu       sync.Mutex
	bins     []int
	min      float64
	max      float64
	labels   []string
	barColor cell.Color
	alertBin int
	alertCol cell.Color
}

// NewHistogram returns a new Histogram widget.
func NewHistogram() (*Histogram, error) {
	return &Histogram{
		barColor: cell.ColorNumber(42), // Greenish
		alertBin: -1,
		alertCol: cell.ColorRed,
	}, nil
}

// SetBins sets the histogram data and bin labels.
func (h *Histogram) SetBins(bins []int, min, max float64, labels []string, alertBin int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.bins = bins
	h.min = min
	h.max = max
	h.labels = labels
	h.alertBin = alertBin
	return nil
}

// SetAlertColor sets the color for alert bins.
func (h *Histogram) SetAlertColor(col cell.Color) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.alertCol = col
}

// Draw draws the Histogram widget onto the canvas.
func (h *Histogram) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.bins) == 0 {
		return nil
	}

	// 1. Setup dimensioni Braille
	ar := cvs.Area()
	brailleW := ar.Dx() * 2
	brailleH := ar.Dy() * 4

	// Padding per evitare che le barre tocchino i bordi
	padding := 4
	plotH := brailleH - (padding * 2)
	plotW := brailleW - (padding * 2)

	if plotW <= 0 || plotH <= 0 {
		return nil
	}

	// 2. Calcola Max Bin per la scala verticale
	maxBin := 0
	for _, v := range h.bins {
		if v > maxBin {
			maxBin = v
		}
	}
	if maxBin == 0 {
		maxBin = 1
	}

	// 3. Calcola larghezza barre
	barCount := len(h.bins)
	// Larghezza in 'pixel' braille per ogni barra
	barWidth := int(math.Max(1, float64(plotW)/float64(barCount)))
	
	// Spaziatura tra le barre (gap): se la barra è abbastanza larga, lasciamo 1px di vuoto
	gap := 0
	if barWidth > 2 {
		gap = 1
	}
	actualBarWidth := barWidth - gap

	// Mappe per memorizzare stato Braille e Colore per ogni cella
	brailleMap := make(map[image.Point]rune)
	colorMap := make(map[image.Point]cell.Color)

	// Helper function per accendere un dot
	setDot := func(bx, by int, col cell.Color) {
		// Coordinate Cella Terminale
		cellX := bx / 2
		cellY := by / 4
		
		subX := bx % 2
		subY := by % 4

		var mask rune
		switch {
		case subX == 0 && subY == 0: mask = 0x01
		case subX == 0 && subY == 1: mask = 0x02
		case subX == 0 && subY == 2: mask = 0x04
		case subX == 0 && subY == 3: mask = 0x40
		case subX == 1 && subY == 0: mask = 0x08
		case subX == 1 && subY == 1: mask = 0x10
		case subX == 1 && subY == 2: mask = 0x20
		case subX == 1 && subY == 3: mask = 0x80
		}

		pt := image.Point{cellX, cellY}
		
		// Aggiorna maschera
		if r, ok := brailleMap[pt]; ok {
			brailleMap[pt] = r | mask
		} else {
			brailleMap[pt] = 0x2800 | mask
		}

		// Aggiorna colore (l'ultimo che scrive vince per la cella intera)
		colorMap[pt] = col
	}

	originY := brailleH - padding
	originX := padding

	// 4. Disegna le barre
	for i, v := range h.bins {
		// Calcola altezza in pixel braille
		height := int((float64(v) / float64(maxBin)) * float64(plotH))
		
		// Seleziona colore
		col := h.barColor
		if h.alertBin == i {
			col = h.alertCol
		}

		// Coordinate X di inizio barra
		startX := originX + (i * barWidth)
		
		// Loop per riempire il rettangolo della barra
		for x := 0; x < actualBarWidth; x++ {
			for y := 0; y < height; y++ {
				// X assoluto nel braille grid
				bx := startX + x
				// Y assoluto (invertito, cresce verso l'alto graficamente)
				by := originY - 1 - y

				// Bounds check
				if bx < brailleW && by >= 0 {
					setDot(bx, by, col)
				}
			}
		}
	}

	// 5. Scrivi sul Canvas
	for pt, r := range brailleMap {
		col := colorMap[pt]
		// Nota: SetCell restituisce (int, error), ignoriamo int con _
		_, err := cvs.SetCell(pt, r, cell.FgColor(col))
		if err != nil {
			continue
		}
	}

	return nil
}

// Keyboard input isn't supported on the Histogram widget.
func (*Histogram) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return errors.New("the Histogram widget doesn't support keyboard events")
}

// Mouse input isn't supported on the Histogram widget.
func (*Histogram) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	return errors.New("the Histogram widget doesn't support mouse events")
}

// Options implements widgetapi.Widget.Options.
func (h *Histogram) Options() widgetapi.Options {
	return widgetapi.Options{
		// Ratio Braille standard 2x4
		Ratio:        image.Point{2, 4},
		MinimumSize:  image.Point{5, 5},
		WantKeyboard: widgetapi.KeyScopeNone,
		WantMouse:    widgetapi.MouseScopeNone,
	}
}