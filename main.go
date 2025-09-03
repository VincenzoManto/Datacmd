package main

import (
	"context"
	"datacmd/generate"
	"datacmd/loader"
	"datacmd/widgets"
	"flag"
	"fmt"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgetapi"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/donut"
	"github.com/mum4k/termdash/widgets/gauge"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/segmentdisplay"
	"github.com/mum4k/termdash/widgets/sparkline"
	"github.com/mum4k/termdash/widgets/text"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

// redrawInterval is how often termdash redraws the screen.
const redrawInterval = 250 * time.Millisecond

// rootID is the ID assigned to the root container.
const rootID = "root"

// Terminal implementations
const (
	termboxTerminal = "termbox"
	tcellTerminal   = "tcell"
)

func main() {
	terminalPtr := flag.String("terminal",
		"tcell",
		"The terminal implementation to use. Available implementations are 'termbox' and 'tcell' (default = tcell).")
	configPath := flag.String("config", "config.yml", "Path to the YAML configuration file.")
	sourcePath := flag.String("source", "", "Path to the data source file or URL.")
	generatePtr := flag.Bool("generate", false, "Generate a dashboard configuration based on the provided source type and path.")
	helpPtr := flag.Bool("help", false, "Show help information.")
	flag.Parse()

	if *helpPtr {
		flag.Usage()
		return
	}

	// if --config is provided load it
	// if --generate is provided, call GenerateDashboardConfig and then load the generated config

	if *generatePtr {
		config, err := generate.GenerateDashboardConfig(*sourcePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating dashboard: %v\n", err)
			os.Exit(1)
		}
		// Generate the YAML file
		yamlData, err := yaml.Marshal(&config)
		if err != nil || yamlData == nil {
			fmt.Fprintf(os.Stderr, "Error generating YAML: %v\n", err)
			os.Exit(1)
		}
		// save it in config.yml
		if err := os.WriteFile("config.yml", yamlData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing YAML to file: %v\n", err)
			os.Exit(1)
		}

	}

	config, csvData, err := loader.LoadConfigAndData(*configPath)
	if err != nil {
		log.Fatalf("Error loading config or data: %v", err)
	}

	var t terminalapi.Terminal
	switch terminal := *terminalPtr; terminal {
	case termboxTerminal:
		t, err = termbox.New(termbox.ColorMode(terminalapi.ColorMode256))
	case tcellTerminal:
		t, err = tcell.New(tcell.ColorMode(terminalapi.ColorMode256))
	default:
		log.Fatalf("Unknown terminal implementation '%s' specified. Choose between 'termbox' and 'tcell'.", terminal)
		return
	}

	if err != nil {
		panic(err)
	}
	defer t.Close()

	c, err := container.New(t, container.ID(rootID))
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Crea i widget dinamicamente in base alla configurazione YAML.
	dynamicWidgets, err := createWidgets(ctx, config, csvData, t)
	if err != nil {
		panic(err)
	}

	// Costruisci il layout in modo dinamico.
	gridOpts, err := dynamicGridLayout(dynamicWidgets, config)
	if err != nil {
		panic(err)
	}

	if err := c.Update(rootID, gridOpts...); err != nil {
		panic(err)
	}

	quitter := func(k *terminalapi.Keyboard) {
		if k.Key == keyboard.KeyEsc || k.Key == keyboard.KeyCtrlC {
			cancel()
		}
	}
	if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter), termdash.RedrawInterval(time.Duration(config.Refresh)*time.Second)); err != nil {
		panic(err)
	}
}

// createWidgets creates a map of widgets based on the YAML configuration.
func createWidgets(ctx context.Context, config *loader.Config, csvData *loader.DataDataSource, t terminalapi.Terminal) (map[string]interface{}, error) {
	widgets := make(map[string]interface{})

	for _, w := range config.Widgets {
		var widget interface{}
		var err error

		// Per semplicità, qui supportiamo solo i tipi di widget presenti nel main.go originale.
		// Altri tipi come heatmap, matrix, pie, radar, scatter richiedono librerie dedicate o implementazioni personalizzate.
		switch w.Type {
		case "sparkline":
			widget, err = createSparkline(ctx, &w, csvData, config.Refresh)
		case "gauge":
			widget, err = createGauge(ctx, &w, csvData, config.Refresh)
		case "line":
			widget, err = createLineChart(ctx, &w, csvData, config.Refresh)
		case "bar":
			widget, err = createBarChart(ctx, &w, csvData, config.Refresh)
		case "donut":
			widget, err = createDonut(ctx, &w, csvData, config.Refresh)
		case "pie":
			widget, err = createPieChart(ctx, &w, csvData, config.Refresh)
		case "text":
			widget, err = createText(ctx, &w, csvData, config.Refresh)
		case "radar":
			widget, err = createRadarChart(ctx, &w, csvData, config.Refresh)
		case "table":
			widget, err = createTable(ctx, &w, csvData, config.Refresh)
		case "funnel":
			widget, err = createFunnel(ctx, &w, csvData, config.Refresh)
		default:
			textWidget, err := text.New()
			if err == nil {
				textWidget.Write(fmt.Sprintf("The widget '%s' is not supported in this version.", w.Type))
			}
			widget = textWidget
			log.Printf("Attention: widget type '%s' is not supported and a message will be displayed.", w.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("Error creating widget '%s': %w", w.Title, err)
		}
		widgets[w.Title] = widget
	}

	// Aggiungi un display per il titolo e un testo di benvenuto statico per mostrare l'uso del widget `text`
	titleText, err := text.New()
	if err != nil {
		return nil, err
	}
	titleText.Write(config.Title, text.WriteCellOpts(cell.FgColor(cell.ColorGreen)))
	widgets["title"] = titleText

	return widgets, nil
}

// dynamicGridLayout builds the grid layout dynamically based on the created widgets.
func dynamicGridLayout(widgets map[string]interface{}, config *loader.Config) ([]container.Option, error) {
	builder := grid.New()

	// Titolo fisso in alto.
	titleWidget, ok := widgets["title"].(widgetapi.Widget)
	if !ok {
		return nil, fmt.Errorf("the title widget is not a valid widget")
	}
	builder.Add(grid.RowHeightPerc(5, grid.Widget(titleWidget, container.Border(linestyle.Light))))

	// Rimuovi il widget del titolo dalla mappa per evitare di processarlo di nuovo
	delete(widgets, "title")

	numWidgets := len(widgets)
	if numWidgets == 0 {
		gridOpts, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return gridOpts, nil
	}

	// Correlate widgets with their configurations to easily access their type.
	type widgetWithConfig struct {
		element grid.Element
		typ     string
		title   string
	}
	widgetList := []widgetWithConfig{}

	// Ensure the order of widgets is consistent with the config.
	widgetConfigs := config.Widgets

	for _, conf := range widgetConfigs {
		widget, ok := widgets[conf.Title].(widgetapi.Widget)
		if !ok {
			return nil, fmt.Errorf("the widget '%s' is not a valid widget", conf.Title)
		}

		opts := []container.Option{
			container.Border(linestyle.Light),
			container.BorderTitle(conf.Title),
		}
		widgetList = append(widgetList, widgetWithConfig{
			element: grid.Widget(widget, opts...),
			typ:     conf.Type,
			title:   conf.Title,
		})
	}

	// Use a slice to build rows dynamically.
	var rows [][]grid.Element
	var currentRow []grid.Element
	currentWidth := 0

	for _, w := range widgetList {
		var widgetWidth int
		switch w.typ {
		case "pie", "donut", "gauge", "radar":
			widgetWidth = 30
		default:
			widgetWidth = 50
		}

		// If adding the new widget exceeds 100%, start a new row.
		if currentWidth+widgetWidth > 100 && len(currentRow) > 0 {

			// Append the new row to the list of rows.
			rows = append(rows, currentRow)

			// Start a new row with the current widget.
			currentRow = []grid.Element{grid.ColWidthPerc(widgetWidth, w.element)}
			currentWidth = widgetWidth
		} else {
			// Add the widget to the current row.
			currentRow = append(currentRow, grid.ColWidthPerc(widgetWidth, w.element))
			currentWidth += widgetWidth
		}
	}

	// Add the last row if it's not empty.
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	// Calculate the height for each row.
	rowHeightPerc := (100 - 5) / len(rows)

	// Add the dynamically created rows to the grid builder.
	for _, row := range rows {
		builder.Add(grid.RowHeightPerc(rowHeightPerc, row...))
	}

	gridOpts, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return gridOpts, nil
}

// periodic executes the provided closure periodically every interval.
func periodic(ctx context.Context, interval time.Duration, fn func() error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func createTable(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*widgets.Table, error) {
	headers := make([]*widgets.Cell, len(csvData.Header))
	for i, header := range csvData.Header {
		headers[i] = widgets.NewCell(header)
	}

	rows := make([][]*widgets.Cell, len(csvData.Records))
	for i, record := range csvData.Records {
		rows[i] = make([]*widgets.Cell, len(record))
		for j, col := range record {
			rows[i][j] = widgets.NewCell(col)
		}
	}

	opts := []widgets.TableOption{
		widgets.CellFillColor(cell.ColorDefault),
		widgets.HeaderFillColor(cell.ColorBlack),
	}

	table, err := widgets.NewTable(headers, rows, opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}
	return table, nil
}

// createSparkline creates and starts a new sparkline widget.
func createSparkline(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*sparkline.SparkLine, error) {
	valueColIndex := -1
	for i, header := range csvData.Header {
		if header == w.ValueCol {
			valueColIndex = i
			break
		}
	}
	if valueColIndex == -1 {
		return nil, fmt.Errorf("column '%s' not found for widget '%s'", w.ValueCol, w.Title)
	}

	sp, err := sparkline.New(sparkline.Color(cell.ColorGreen))
	if err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		var values []int
		for _, record := range csvData.Records {
			val, err := strconv.Atoi(record[valueColIndex])
			if err != nil {
				continue
			}
			values = append(values, val)
		}
		return sp.Add(values)
	})
	return sp, nil
}

// createGauge creates and starts a new gauge widget.
func createGauge(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*gauge.Gauge, error) {
	valueColIndex := -1
	for i, header := range csvData.Header {
		if header == w.ValueCol {
			valueColIndex = i
			break
		}
	}
	if valueColIndex == -1 {
		return nil, fmt.Errorf("column '%s' not found for widget '%s'", w.ValueCol, w.Title)
	}

	g, err := gauge.New()
	if err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		if len(csvData.Records) > 0 {
			val, err := strconv.Atoi(csvData.Records[len(csvData.Records)-1][valueColIndex])
			if err == nil {
				return g.Percent(val)
			}
		}
		return nil
	})
	return g, nil
}

// createLineChart creates and starts a new line chart widget.
func createLineChart(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*linechart.LineChart, error) {
	xColIndex, yColIndex := -1, -1
	for i, header := range csvData.Header {
		if header == w.XCol {
			xColIndex = i
		}
		if header == w.YCol {
			yColIndex = i
		}
	}
	if xColIndex == -1 || yColIndex == -1 {
		return nil, fmt.Errorf("column 'x_col' or 'y_col' not found for widget '%s'", w.Title)
	}

	lc, err := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorRed)),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorGreen)),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorGreen)),
	)
	if err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		var inputs []float64
		xLabels := make(map[int]string)
		for i, record := range csvData.Records {
			val, err := strconv.ParseFloat(record[yColIndex], 64)
			if err != nil {
				continue
			}
			inputs = append(inputs, val)
			xLabels[i] = record[xColIndex]
		}
		return lc.Series(w.Title, inputs,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(42))),
			linechart.SeriesXLabels(xLabels),
		)
	})
	return lc, nil
}

// createBarChart creates and starts a new bar chart widget.
func createBarChart(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*barchart.BarChart, error) {
	xColIndex, yColIndex := -1, -1
	for i, header := range csvData.Header {
		if header == w.XCol {
			xColIndex = i
		}
		if header == w.YCol {
			yColIndex = i
		}
	}
	if xColIndex == -1 || yColIndex == -1 {
		return nil, fmt.Errorf("column 'x_col' or 'y_col' not found for widget '%s'", w.Title)
	}

	bc, err := barchart.New(
		barchart.ShowValues(),
		// Questa è la riga corretta che fornisce un slice di colori.
		barchart.BarColors([]cell.Color{cell.ColorNumber(42)}),
	)
	if err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		var values []int
		for _, record := range csvData.Records {
			val, err := strconv.Atoi(record[yColIndex])
			if err != nil {
				continue
			}
			values = append(values, val)
		}
		return bc.Values(values, 100)
	})

	return bc, nil
}

// createDonut creates and starts a new donut widget.
func createDonut(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*donut.Donut, error) {
	valueColIndex := -1
	for i, header := range csvData.Header {
		if header == w.ValueCol {
			valueColIndex = i
			break
		}
	}
	if valueColIndex == -1 {
		return nil, fmt.Errorf("column '%s' not found for widget '%s'", w.ValueCol, w.Title)
	}

	d, err := donut.New()
	if err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		if len(csvData.Records) > 0 {
			val, err := strconv.Atoi(csvData.Records[len(csvData.Records)-1][valueColIndex])
			if err == nil {
				return d.Percent(val)
			}
		}
		return nil
	})
	return d, nil
}

func createPieChart(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*widgets.PieChart, error) {
	valueColIndex := -1
	for i, header := range csvData.Header {
		if header == w.ValueCol {
			valueColIndex = i
			break
		}
	}
	if valueColIndex == -1 {
		return nil, fmt.Errorf("column '%s' not found for widget '%s'", w.ValueCol, w.Title)
	}

	pc, err := widgets.NewPieChart()
	if err != nil {
		return nil, err
	}

	// Leggi i dati per le fette della torta
	var values []int
	for _, record := range csvData.Records {
		val, err := strconv.Atoi(record[valueColIndex])
		if err != nil {
			continue
		}
		values = append(values, val)
	}

	// Definisci i colori per le fette. Devi specificarne uno per ogni fetta.
	// Se hai più fette che colori, i colori si ripeteranno.
	colors := []cell.Color{
		cell.ColorNumber(42),
		cell.ColorNumber(197),
		cell.ColorNumber(214),
		cell.ColorNumber(255),
		cell.ColorNumber(39),
		cell.ColorNumber(45),
		cell.ColorNumber(51),
		cell.ColorNumber(57),
		cell.ColorNumber(63),
	}

	if err := pc.Values(values, colors); err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		return nil
	})

	return pc, nil
}

func createText(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*segmentdisplay.SegmentDisplay, error) {

	t, err := segmentdisplay.New()
	if err != nil {
		return nil, err
	}
	// print the value of aggregation based on the value_col
	valueColIndex := -1
	for i, header := range csvData.Header {
		if header == w.ValueCol {
			valueColIndex = i
			break
		}
	}
	if valueColIndex == -1 {
		return nil, fmt.Errorf("colonna '%s' non trovata per il widget '%s'", w.ValueCol, w.Title)
	}
	if len(csvData.Records) > 0 {
		var values []int
		for _, record := range csvData.Records {
			val, err := strconv.Atoi(record[valueColIndex])
			if err == nil {
				values = append(values, val)
			}
		}

		if len(values) == 0 {
			rollText(ctx, t, fmt.Sprintf("%s: No valid data", w.Title))
			return t, nil
		}

		var result string
		switch w.Aggregation {
		case "sum":
			sum := 0
			for _, v := range values {
				sum += v
			}
			result = fmt.Sprintf("Sum: %d", sum)
		case "avg":
			sum := 0
			for _, v := range values {
				sum += v
			}
			avg := float64(sum) / float64(len(values))
			result = fmt.Sprintf("Avg: %.2f", avg)
		case "median":
			sort.Ints(values)
			median := 0
			if len(values)%2 == 0 {
				median = (values[len(values)/2-1] + values[len(values)/2]) / 2
			} else {
				median = values[len(values)/2]
			}
			result = fmt.Sprintf("Median: %d", median)
		case "max":
			max := values[0]
			for _, v := range values {
				if v > max {
					max = v
				}
			}
			result = fmt.Sprintf("Max: %d", max)
		case "min":
			min := values[0]
			for _, v := range values {
				if v < min {
					min = v
				}
			}
			result = fmt.Sprintf("Min: %d", min)
		default:
			result = "Invalid aggregation type"
		}

		rollText(ctx, t, result)
	}
	return t, nil
}

func rollText(ctx context.Context, sd *segmentdisplay.SegmentDisplay, text string) {
	var chunks []*segmentdisplay.TextChunk
	chunks = append(chunks, segmentdisplay.NewChunk(
		text,
		segmentdisplay.WriteCellOpts(cell.FgColor(cell.ColorNumber(42))),
	))
	if err := sd.Write(chunks); err != nil {
		panic(err)
	}
}

func createRadarChart(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*widgets.Radar, error) {
	data := make(map[string]float64)
	for _, record := range csvData.Records {
		label := record[0]
		value, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			continue
		}
		data[label] = value
	}

	max := 0.0
	for _, value := range data {
		if value > max {
			max = value
		}
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no valid data found for radar chart")
	}

	values := &widgets.Values{
		Data: data,
		Max:  max,
	}

	r, err := widgets.NewRadar()
	if err != nil {
		return nil, err
	}

	if err := r.SetValues(values); err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		return nil
	})

	return r, nil
}

func createFunnel(ctx context.Context, w *loader.WidgetConfig, csvData *loader.DataDataSource, refresh int) (*widgets.Funnel, error) {
	data := make([]int, 0)
	colors := make([]cell.Color, 0)

	for _, record := range csvData.Records {
		value, err := strconv.Atoi(record[1])
		if err != nil {
			continue
		}
		data = append(data, value)
		colors = append(colors, cell.ColorNumber(len(colors)+1))
	}

	funnel, err := widgets.NewFunnel()
	if err != nil {
		return nil, err
	}

	if err := funnel.Values(data, colors); err != nil {
		return nil, err
	}

	go periodic(ctx, time.Duration(refresh)*time.Second, func() error {
		return nil
	})

	return funnel, nil
}
