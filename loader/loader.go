package loader

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Title   string         `yaml:"title"`
	Refresh int            `yaml:"refresh"`
	Source  Source         `yaml:"source"`
	Widgets []WidgetConfig `yaml:"widgets"`
}

type WidgetConfig struct {
	Type        string `yaml:"type"`
	Title       string `yaml:"title"`
	ValueCol    string `yaml:"value_col,omitempty"`
	LabelCol    string `yaml:"label_col,omitempty"`
	XCol        string `yaml:"x_col,omitempty"`
	YCol        string `yaml:"y_col,omitempty"`
	ZCol        string `yaml:"z_col,omitempty"`
	CatCol      string `yaml:"cat_col,omitempty"`
	Aggregation string `yaml:"aggregation,omitempty"`
	MaxValue    int    `yaml:"max_value,omitempty"`
}

type Source struct {
	Type string `yaml:"type"`
	Path string `yaml:"path"`
	URL  string `yaml:"url"`
}

type DataDataSource struct {
	Header  []string
	Records [][]string
}

type DataSource interface {
	Load() (*DataDataSource, error)
}

type CSVDataSource struct {
	Path string
}

func (c *CSVDataSource) Load() (*DataDataSource, error) {
	file, err := os.Open(c.Path)
	if err != nil {
		return nil, fmt.Errorf("Unable to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("Unable to read CSV file: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	header := records[0]
	var data DataDataSource
	data.Header = header
	data.Records = make([][]string, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) != len(header) {
			return nil, fmt.Errorf("record with number of columns not matching header: %v", record)
		}
		data.Records = append(data.Records, record)
	}
	return &data, nil
}

type JSONDataSource struct {
	Path string
}

func (j *JSONDataSource) Load() (*DataDataSource, error) {
	fileData, err := os.ReadFile(j.Path)
	if err != nil {
		return nil, fmt.Errorf("Unable to read JSON file: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, fmt.Errorf("Unable to parse JSON file: %w", err)
	}
	return &data, nil
}

type APIDataSource struct {
	URL string
}

func (a *APIDataSource) Load() (*DataDataSource, error) {
	resp, err := http.Get(a.URL)
	if err != nil {
		return nil, fmt.Errorf("Unable to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API response failed, status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read response body: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("Unable to decode API JSON response: %w", err)
	}
	return &data, nil
}

// SystemMetricsDataSource handles loading system metrics.
type SystemMetricsDataSource struct{}

func (s *SystemMetricsDataSource) Load() (*DataDataSource, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("Unable to get virtual memory: %w", err)
	}

	c, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("Unable to get CPU usage: %w", err)
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

func LoadConfigAndData(configPath string) (*Config, *DataDataSource, error) {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, nil, fmt.Errorf("Unable to parse YAML config file: %w", err)
	}

	var dataSource DataSource
	switch config.Source.Type {
	case "csv":
		dataSource = &CSVDataSource{Path: config.Source.Path}
	case "json":
		dataSource = &JSONDataSource{Path: config.Source.Path}
	case "api":
		dataSource = &APIDataSource{URL: config.Source.URL}
	case "system":
		dataSource = &SystemMetricsDataSource{}
	default:
		return nil, nil, fmt.Errorf("Unsupported data source type: %s", config.Source.Type)
	}

	data, err := dataSource.Load()
	if err != nil {
		return nil, nil, err
	}

	return &config, data, nil
}
