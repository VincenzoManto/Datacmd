package generate

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// --- Structs for YAML configuration generation ---

// Config holds the dashboard configuration.
type Config struct {
	Title   string `yaml:"title"`
	Refresh int    `yaml:"refresh"`
	Source  Source `yaml:"source"`
	Widgets []WidgetConfig `yaml:"widgets"`
}

// Source holds the data source details.
type Source struct {
	Type string `yaml:"type"`
	Path string `yaml:"path,omitempty"`
	URL  string `yaml:"url,omitempty"`
}

// WidgetConfig holds the configuration for a single widget.
type WidgetConfig struct {
	Type        string `yaml:"type"`
	Title       string `yaml:"title"`
	ValueCol    string `yaml:"value_col,omitempty"`
	LabelCol    string `yaml:"label_col,omitempty"`
	XCol        string `yaml:"x_col,omitempty"`
	YCol        string `yaml:"y_col,omitempty"`
	CatCol      string `yaml:"cat_col,omitempty"`
	Aggregation string `yaml:"aggregation,omitempty"`
	Columns     []TableColumn `yaml:"columns,omitempty"`
}

// TableColumn is used for the table widget to define column display.
type TableColumn struct {
	Title     string `yaml:"title"`
	DataIndex string `yaml:"dataIndex"`
}

// --- Interfaces and implementations for data loading (based on your loader) ---

// DataDataSource holds the loaded data.
type DataDataSource struct {
	Header  []string
	Records [][]string
}

// DataSource defines the contract for any data source.
type DataSource interface {
	Load() (*DataDataSource, error)
}

// CSVDataSource handles loading data from a CSV file.
type CSVDataSource struct {
	Path string
}

func (c *CSVDataSource) Load() (*DataDataSource, error) {
	file, err := os.Open(c.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("unable to read CSV file: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	header := records[0]
	data := DataDataSource{Header: header, Records: records[1:]}
	return &data, nil
}

// JSONDataSource handles loading data from a JSON file.
type JSONDataSource struct {
	Path string
}

func (j *JSONDataSource) Load() (*DataDataSource, error) {
	fileData, err := os.ReadFile(j.Path)
	if err != nil {
		return nil, fmt.Errorf("unable to read JSON file: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, fmt.Errorf("unable to parse JSON file: %w", err)
	}
	return &data, nil
}

// APIDataSource handles loading data from an API.
type APIDataSource struct {
	URL string
}

func (a *APIDataSource) Load() (*DataDataSource, error) {
	resp, err := http.Get(a.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API response failed, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("unable to decode API JSON response: %w", err)
	}
	return &data, nil
}

// SystemMetricsDataSource handles loading system metrics.
type SystemMetricsDataSource struct{}

func (s *SystemMetricsDataSource) Load() (*DataDataSource, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("unable to get virtual memory: %w", err)
	}

	c, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("unable to get CPU usage: %w", err)
	}

	data := DataDataSource{
		Header: []string{"metric_name", "value"},
		Records: [][]string{
			{"memory_total_bytes", fmt.Sprintf("%v", v.Total)},
			{"memory_used_percent", fmt.Sprintf("%v", v.UsedPercent)},
			{"cpu_usage_percent", fmt.Sprintf("%v", c[0])},
		},
	}
	return &data, nil
}

// --- Main logic of autogen.go ---

// isNumeric checks if a string can be parsed as a float.
func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// GenerateDashboardConfig generates a dashboard configuration based on the provided source.
func GenerateDashboardConfig(sourcePath string) (*Config, error) {

	// Evinct type from path
	var sourceType string
	if strings.HasSuffix(sourcePath, ".csv") {
		sourceType = "csv"
	} else if strings.HasSuffix(sourcePath, ".json") {
		sourceType = "json"
	} else if strings.HasPrefix(sourcePath, "http://") || strings.HasPrefix(sourcePath, "https://") {
		sourceType = "api"
	} else {
		sourceType = "system"
	}
	// Create the data source instance
	var dataSource DataSource
	var sourceTitle string
	switch sourceType {
	case "csv":
		if sourcePath == "" {
			return nil, fmt.Errorf("error: path is required for 'csv' type")
		}
		dataSource = &CSVDataSource{Path: sourcePath}
		sourceTitle = "Dashboard for " + sourcePath
	case "json":
		if sourcePath == "" {
			return nil, fmt.Errorf("error: path is required for 'json' type")
		}
		dataSource = &JSONDataSource{Path: sourcePath}
		sourceTitle = "Dashboard for " + sourcePath
	case "api":
		if sourcePath == "" {
			return nil, fmt.Errorf("error: URL is required for 'api' type")
		}
		dataSource = &APIDataSource{URL: sourcePath}
		sourceTitle = "Dashboard for " + sourcePath
	case "system":
		dataSource = &SystemMetricsDataSource{}
		sourceTitle = "System Metrics Dashboard"
	default:
		return nil, fmt.Errorf("error: unsupported data source type: %s", sourceType)
	}

	// Load the data
	data, err := dataSource.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading data: %w", err)
	}

	// Column analysis (numeric vs. categorical)
	numericCols := make(map[string]bool)
	var firstNumericCol string
	var firstCategoricCol string

	for _, header := range data.Header {
		isNum := false
		if len(data.Records) > 0 {
			// Check the first 5 records to determine the column type
			for i := 0; i < 5 && i < len(data.Records); i++ {
				colIndex := -1
				for j, h := range data.Header {
					if h == header {
						colIndex = j
						break
					}
				}
				if colIndex == -1 {
					continue
				}
				if isNumeric(data.Records[i][colIndex]) {
					isNum = true
				} else {
					isNum = false
					break
				}
			}
		}
		numericCols[header] = isNum
		if isNum && firstNumericCol == "" {
			firstNumericCol = header
		}
		if !isNum && firstCategoricCol == "" {
			firstCategoricCol = header
		}
	}

	// Configuration generation
	var widgets []WidgetConfig

	// Table Widget (always present)
	tableCols := []TableColumn{}
	for _, header := range data.Header {
		tableCols = append(tableCols, TableColumn{
			Title:     strings.Title(strings.ReplaceAll(header, "_", " ")),
			DataIndex: header,
		})
	}
	widgets = append(widgets, WidgetConfig{
		Type:    "table",
		Title:   "Table",
		Columns: tableCols,
	})

	// Generate widgets based on data analysis
	for _, header := range data.Header {
		isNum := numericCols[header]

		if isNum {
			// Widgets for numeric columns
			// If there is a categorical column, create charts that use it as a label
			if firstCategoricCol != "" {
				widgets = append(widgets, WidgetConfig{
					Type:     "pie",
					Title:    fmt.Sprintf("Pie Chart (%s)", header),
					ValueCol: header,
					LabelCol: firstCategoricCol,
				})
				widgets = append(widgets, WidgetConfig{
					Type:     "bar",
					Title:    fmt.Sprintf("Bar Chart (%s)", header),
					XCol:     firstCategoricCol,
					YCol:     header,
				})
				widgets = append(widgets, WidgetConfig{
					Type:     "line",
					Title:    fmt.Sprintf("Line Chart (%s)", header),
					XCol:     firstCategoricCol,
					YCol:     header,
				})
				widgets = append(widgets, WidgetConfig{
					Type:     "radar",
					Title:    fmt.Sprintf("Radar Chart (%s)", header),
					CatCol:   firstCategoricCol,
					ValueCol: header,
				})
			}

			// Widgets without dependency on categorical columns
			widgets = append(widgets, WidgetConfig{
				Type:     "gauge",
				Title:    fmt.Sprintf("Gauge (%s)", header),
				ValueCol: header,
			})
			widgets = append(widgets, WidgetConfig{
				Type:        "text",
				Title:       fmt.Sprintf("Text (%s - Sum)", header),
				ValueCol:    header,
				Aggregation: "sum",
			})
		}
	}

	// Create the final configuration object
	config := &Config{
		Title:   sourceTitle,
		Refresh: 5,
		Source: Source{
			Type: sourceType,
			Path: sourcePath,
		},
		Widgets: widgets,
	}

	return config, nil
}
