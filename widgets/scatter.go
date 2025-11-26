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

// ScatterPlot displays a scatter plot of two numeric columns.
type ScatterPlot struct {
	mu     sync.Mutex
	points []ScatterPoint
	xLabel string
	yLabel string
	color  cell.Color
}

type ScatterPoint struct {
	X float64
	Y float64
}

// NewScatterPlot returns a new ScatterPlot widget.
func NewScatterPlot() (*ScatterPlot, error) {
	// Usa un colore predefinito (es. Ciano)
	return &ScatterPlot{color: cell.ColorNumber(45)}, nil
}

// SetPoints sets the data for the scatter plot.
func (s *ScatterPlot) SetPoints(points []ScatterPoint, xLabel, yLabel string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points = points
	s.xLabel = xLabel
	s.yLabel = yLabel
	return nil
}

// Draw draws the ScatterPlot widget onto the canvas.
func (s *ScatterPlot) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.points) == 0 {
		return nil
	}

	// 1. Calcolo Min/Max manuale per evitare dipendenze complesse
	minX, maxX := math.MaxFloat64, -math.MaxFloat64
	minY, maxY := math.MaxFloat64, -math.MaxFloat64

	for _, pt := range s.points {
		minX = math.Min(minX, pt.X)
		maxX = math.Max(maxX, pt.X)
		minY = math.Min(minY, pt.Y)
		maxY = math.Max(maxY, pt.Y)
	}

	// Evita divisione per zero se tutti i punti sono identici
	if maxX == minX {
		maxX += 1
		minX -= 1
	}
	if maxY == minY {
		maxY += 1
		minY -= 1
	}

	// 2. Setup area e risoluzione Braille (2x4 punti per cella)
	ar := cvs.Area()
	// La risoluzione Braille è 2 volte la larghezza e 4 volte l'altezza in celle
	brailleW := ar.Dx() * 2
	brailleH := ar.Dy() * 4
	
	// Margine (padding) in "sottopixel" braille
	padding := 4 
	plotW := brailleW - (padding * 2)
	plotH := brailleH - (padding * 2)

	if plotW <= 0 || plotH <= 0 {
		return nil // Area troppo piccola per disegnare
	}

	// Mappa locale per accumulare i punti Braille (coordinate cella -> runa braille)
	brailleMap := make(map[image.Point]rune)

	// Funzione helper per accendere un singolo "dot" braille
	setDot := func(bx, by int) {
		// Coordinate della cella nel terminale
		cellX := bx / 2
		cellY := by / 4
		
		// Coordinate del punto all'interno della cella (0-1, 0-3)
		subX := bx % 2
		subY := by % 4

		// Maschere bit per i caratteri Braille Unicode (ISO/IEC 10646)
		// Pattern:
		// 1 4
		// 2 5
		// 3 6
		// 7 8
		var mask rune
		switch {
		case subX == 0 && subY == 0: mask = 0x01
		case subX == 0 && subY == 1: mask = 0x02
		case subX == 0 && subY == 2: mask = 0x04
		case subX == 0 && subY == 3: mask = 0x40 // Dot 7
		case subX == 1 && subY == 0: mask = 0x08
		case subX == 1 && subY == 1: mask = 0x10
		case subX == 1 && subY == 2: mask = 0x20
		case subX == 1 && subY == 3: mask = 0x80 // Dot 8
		}

		p := image.Point{X: cellX, Y: cellY}
		if r, ok := brailleMap[p]; ok {
			brailleMap[p] = r | mask
		} else {
			brailleMap[p] = 0x2800 | mask // 0x2800 è il carattere braille vuoto
		}
	}

	// Origine del grafico (in basso a sinistra visivamente)
	// Nota: brailleH è il fondo perché le coordinate Y crescono verso il basso
	originX := padding
	originY := brailleH - padding

	// 3. Disegna gli assi cartesiani (L-Shape)
	// Asse Y (Verticale)
	for y := padding; y < originY; y++ {
		setDot(originX, y)
	}
	// Asse X (Orizzontale)
	for x := originX; x < brailleW-padding; x++ {
		setDot(x, originY)
	}

	// 4. Mappa e disegna i punti dei dati
	for _, pt := range s.points {
		xNorm := (pt.X - minX) / (maxX - minX)
		yNorm := (pt.Y - minY) / (maxY - minY)

		// Calcolo coordinata braille X
		bx := originX + int(xNorm*float64(plotW))
		
		// Calcolo coordinata braille Y (invertita perché 0 è in alto)
		by := originY - int(yNorm*float64(plotH))

		// Controllo limiti (bounds check)
		if bx >= 0 && bx < brailleW && by >= 0 && by < brailleH {
			setDot(bx, by)
		}
	}

	// 5. Scrittura finale sul Canvas di Termdash
	for p, r := range brailleMap {
		// CORREZIONE QUI: SetCell restituisce (int, error), ignoriamo l'int.
		_, err := cvs.SetCell(p, r, cell.FgColor(s.color))
		if err != nil {
			// Ignora errori se proviamo a scrivere fuori area (clipping)
			continue
		}
	}

	return nil
}

// Keyboard input isn't supported on the ScatterPlot widget.
func (*ScatterPlot) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return errors.New("the ScatterPlot widget doesn't support keyboard events")
}

// Mouse input isn't supported on the ScatterPlot widget.
func (*ScatterPlot) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	return errors.New("the ScatterPlot widget doesn't support mouse events")
}

// Options implements widgetapi.Widget.Options.
func (s *ScatterPlot) Options() widgetapi.Options {
	return widgetapi.Options{
		// Ratio suggerito 2:4 per mantenere le proporzioni del braille
		Ratio:        image.Point{2, 4}, 
		MinimumSize:  image.Point{5, 5},
		WantKeyboard: widgetapi.KeyScopeNone,
		WantMouse:    widgetapi.MouseScopeNone,
	}
}