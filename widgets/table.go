package widgets

import (
	"fmt"
	"image"
	"math"
	"strings"
	"sync"

	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/private/canvas"
	"github.com/mum4k/termdash/private/draw"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
	"github.com/mum4k/termdash/widgets/button"
)

// Cell is a part of or the full text displayed in a table cell.
type Cell struct {
	text string
}

// NewCell creates a new table cell.
func NewCell(text string) *Cell {
	return &Cell{
		text: text,
	}
}

// Table displays data in a grid of rows and columns.
type Table struct {
	
	headers []*Cell
	
	rows [][]*Cell

	
	mu sync.Mutex
	
	opts *tableOptions

	
	currentPage int
	
	rowsPerPage int
	
	numPages int

	
	prevButton *button.Button
	nextButton *button.Button

	
	prevButtonRect image.Rectangle
	nextButtonRect image.Rectangle
}

// NewTable returns a new Table widget.
// The headers and rows can be nil or empty, but must contain the same number
// of columns.
func NewTable(headers []*Cell, rows [][]*Cell, opts ...TableOption) (*Table, error) {
	numCols := 0
	if len(headers) > 0 {
		numCols = len(headers)
	} else if len(rows) > 0 {
		numCols = len(rows[0])
	}

	for _, row := range rows {
		if len(row) != numCols {
			return nil, fmt.Errorf("all rows must have the same number of columns as the headers, expected %d, got %d", numCols, len(row))
		}
	}
	opt := newTableOptions()
	for _, o := range opts {
		o.set(opt)
	}

	numRows := len(rows)
	numPages := 0
	if numRows > 0 && opt.rowsPerPage > 0 {
		numPages = int(math.Ceil(float64(numRows) / float64(opt.rowsPerPage)))
	}
	if numPages == 0 && numRows > 0 {
		numPages = 1
	}

	t := &Table{
		headers:     headers,
		rows:        rows,
		opts:        opt,
		currentPage: 0,
		rowsPerPage: opt.rowsPerPage,
		numPages:    numPages,
	}

	var err error
	// The buttons now call a method on the table itself to signal that a page change occurred.
	t.prevButton, err = button.New("Prev", func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.prevPage()
		return nil
	})
	if err != nil {
		return nil, err
	}

	t.nextButton, err = button.New("Next", func() error {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.nextPage()
		return nil
	})
	if err != nil {
		return nil, err
	}

	return t, nil
}

// Draw draws the Table widget onto the canvas.
// Implements widgetapi.Widget.Draw.
func (t *Table) Draw(cvs *canvas.Canvas, meta *widgetapi.Meta) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cvsAr := cvs.Area()

	numCols := len(t.headers)
	if numCols == 0 && len(t.rows) > 0 {
		numCols = len(t.rows[0])
	}
	if numCols == 0 {
		return fmt.Errorf("cannot draw table without headers or rows")
	}

	// Use a fixed column width for simplicity.
	colWidth := cvsAr.Dx() / numCols
	if colWidth == 0 {
		return fmt.Errorf("not enough space to draw the table")
	}

	curY := 0
	// Draw headers
	if len(t.headers) > 0 {
		if err := t.drawRow(cvs, cvsAr, t.headers, colWidth, &curY, true); err != nil {
			return err
		}
	}

	// Calculate the slice of rows for the current page.
	startIndex := t.currentPage * t.rowsPerPage
	endIndex := startIndex + t.rowsPerPage
	if endIndex > len(t.rows) {
		endIndex = len(t.rows)
	}

	// Handle the case where the current page has no content.
	// This can happen if rows are removed, so we reset the page.
	if startIndex >= len(t.rows) && len(t.rows) > 0 {
		t.currentPage = t.numPages - 1
		startIndex = t.currentPage * t.rowsPerPage
		endIndex = startIndex + t.rowsPerPage
		if endIndex > len(t.rows) {
			endIndex = len(t.rows)
		}
	}

	// Draw rows for the current page
	for _, row := range t.rows[startIndex:endIndex] {
		if err := t.drawRow(cvs, cvsAr, row, colWidth, &curY, false); err != nil {
			return err
		}
	}

	// Draw pagination indicator and buttons at the bottom.
	if t.numPages > 1 {
		prevButtonText := " < Prev"
		nextButtonText := "Next > "
		prevButtonWidth := len(prevButtonText)
		nextButtonWidth := len(nextButtonText)

		// Calculate the area for the pagination row.
		paginationAr := image.Rect(cvsAr.Min.X, cvsAr.Max.Y-1, cvsAr.Max.X, cvsAr.Max.Y)

		// Draw prev button manually with styling
		t.prevButtonRect = image.Rect(paginationAr.Min.X, paginationAr.Min.Y, paginationAr.Min.X+prevButtonWidth, paginationAr.Max.Y)
		if err := draw.Text(cvs, prevButtonText, t.prevButtonRect.Min,
			draw.TextCellOpts(cell.BgColor(cell.ColorBlue), cell.FgColor(cell.ColorWhite)),
		); err != nil {
			return err
		}

		// Draw next button manually with styling
		t.nextButtonRect = image.Rect(paginationAr.Max.X-nextButtonWidth, paginationAr.Min.Y, paginationAr.Max.X, paginationAr.Max.Y)
		if err := draw.Text(cvs, nextButtonText, t.nextButtonRect.Min,
			draw.TextCellOpts(cell.BgColor(cell.ColorBlue), cell.FgColor(cell.ColorWhite)),
		); err != nil {
			return err
		}

		// Draw pagination indicator text.
		pageIndicator := fmt.Sprintf("Page %d of %d", t.currentPage+1, t.numPages)
		textAr := image.Rect(t.prevButtonRect.Max.X, cvsAr.Max.Y-1, t.nextButtonRect.Min.X, cvsAr.Max.Y)
		if err := draw.Text(cvs, pageIndicator,
			image.Point{X: (textAr.Min.X + textAr.Max.X) / 2, Y: textAr.Max.Y - 1},
			draw.TextCellOpts(cell.BgColor(t.opts.pageIndicatorBgColor), cell.FgColor(t.opts.pageIndicatorFgColor)),
		); err != nil {
			return err
		}

	} else {
		// If there is no pagination, clear the area where buttons would be.
		bottomAr := image.Rect(cvsAr.Min.X, cvsAr.Max.Y-1, cvsAr.Max.X, cvsAr.Max.Y)
		cvs.SetAreaCells(bottomAr, ' ', cell.BgColor(cell.ColorDefault))
	}

	return nil
}

func (t *Table) drawRow(cvs *canvas.Canvas, cvsAr image.Rectangle, row []*Cell, colWidth int, curY *int, isHeader bool) error {
	rowAr := image.Rect(cvsAr.Min.X, *curY, cvsAr.Max.X, *curY+1)
	if rowAr.Dy() == 0 {
		return nil // Avoid drawing on an area with zero height
	}

	fillColor := t.opts.cellFillColor
	textColor := t.opts.cellTextColor
	if isHeader {
		fillColor = t.opts.headerFillColor
		textColor = t.opts.headerTextColor
	}

	if err := cvs.SetAreaCells(rowAr, ' ', cell.BgColor(fillColor)); err != nil {
		return err
	}

	curX := 0
	for _, c := range row {
		// Calculate the column area with a small padding
		colAr := image.Rect(curX+1, rowAr.Min.Y, curX+colWidth-1, rowAr.Max.Y)

		// Now we draw directly to the main canvas.
		text := c.text
		if isHeader {
			text = strings.ToUpper(text)
		}

		// Draw text to the sub-canvas
		if err := draw.Text(cvs, text, colAr.Min,
			draw.TextCellOpts(cell.FgColor(textColor)),
		); err != nil {
			return err
		}

		curX += colWidth
	}
	*curY++
	return nil
}

// Keyboard implements widgetapi.Widget.Keyboard.
func (t *Table) Keyboard(k *terminalapi.Keyboard, meta *widgetapi.EventMeta) error {
	return nil
}

// nextPage advances to the next page.
func (t *Table) nextPage() {
	if t.numPages <= 1 {
		return
	}
	if (t.currentPage + 1) >= t.numPages {
		return
	}
	t.currentPage++
}

// prevPage goes back to the previous page.
func (t *Table) prevPage() {
	if t.numPages <= 1 {
		return
	}
	if t.currentPage <= 0 {
		return
	}
	t.currentPage--
}

func (t *Table) Mouse(m *terminalapi.Mouse, meta *widgetapi.EventMeta) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if m.Button > 0 {
		if m.Position.In(t.prevButtonRect) {
			t.prevPage()
			return nil
		}
		if m.Position.In(t.nextButtonRect) {
			t.nextPage()
			return nil
		}
	}

	return nil
}

// Options implements widgetapi.Widget.Options.
func (t *Table) Options() widgetapi.Options {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Calculate the minimum size based on headers and rows.
	numCols := len(t.headers)
	if numCols == 0 && len(t.rows) > 0 {
		numCols = len(t.rows[0])
	}

	minWidth := numCols * t.opts.minColWidth
	minHeight := t.rowsPerPage
	if len(t.headers) > 0 {
		minHeight++ // Add space for headers
	}

	// Add space for the buttons and the page indicator.
	if t.numPages > 1 {
		minHeight++ // +1 for the button/indicator row
	}

	return widgetapi.Options{
		MinimumSize:  image.Point{minWidth, minHeight},
		WantKeyboard: widgetapi.KeyScopeNone, // Buttons are handled by mouse.
		WantMouse:    widgetapi.MouseScopeGlobal,
	}
}

// tableOptions for the Table widget.
type tableOptions struct {
	minColWidth          int
	rowsPerPage          int
	cellFillColor        cell.Color
	cellTextColor        cell.Color
	headerFillColor      cell.Color
	headerTextColor      cell.Color
	pageIndicatorBgColor cell.Color
	pageIndicatorFgColor cell.Color
}

// newTableOptions returns a new tableOptions struct with default values.
func newTableOptions() *tableOptions {
	return &tableOptions{
		minColWidth:          10,
		rowsPerPage:          5, // Default to 5 rows per page
		cellFillColor:        cell.ColorDefault,
		cellTextColor:        cell.ColorDefault,
		headerFillColor:      cell.ColorBlack,
		headerTextColor:      cell.ColorWhite,
		pageIndicatorBgColor: cell.ColorBlack,
		pageIndicatorFgColor: cell.ColorWhite,
	}
}

// TableOption is used to configure the Table widget.
type TableOption interface {
	set(*tableOptions)
}

// tableOption implements the TableOption interface.
type tableOption func(*tableOptions)

func (o tableOption) set(opts *tableOptions) {
	o(opts)
}

// CellFillColor sets the background color of the table cells.
func CellFillColor(c cell.Color) TableOption {
	return tableOption(func(opts *tableOptions) {
		opts.cellFillColor = c
	})
}

// HeaderFillColor sets the background color of the table headers.
func HeaderFillColor(c cell.Color) TableOption {
	return tableOption(func(opts *tableOptions) {
		opts.headerFillColor = c
	})
}

// RowsPerPage sets the number of rows to display per page.
func RowsPerPage(count int) TableOption {
	return tableOption(func(opts *tableOptions) {
		if count > 0 {
			opts.rowsPerPage = count
		}
	})
}
