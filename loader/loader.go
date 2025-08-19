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

// Config holds the dashboard configuration from a YAML file.
type Config struct {
	Title   string `yaml:"title"`
	Refresh int    `yaml:"refresh"`
	Source  Source `yaml:"source"`
	Widgets []WidgetConfig `yaml:"widgets"`
}

// WidgetConfig holds the configuration for a single widget.
type WidgetConfig struct {
	Type     string `yaml:"type"`
	Title    string `yaml:"title"`
	ValueCol string `yaml:"value_col,omitempty"`
	LabelCol string `yaml:"label_col,omitempty"`
	XCol     string `yaml:"x_col,omitempty"`
	YCol     string `yaml:"y_col,omitempty"`
	ZCol     string `yaml:"z_col,omitempty"` // Per i grafici non supportati in questa versione, ma per completezza.
	CatCol   string `yaml:"cat_col,omitempty"` // Per i grafici non supportati in questa versione, ma per completezza.
	Aggregation string `yaml:"aggregation,omitempty"` // Tipo di aggregazione (max, min, avg)
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

// --- Definizione dell'interfaccia DataSource ---
// DataSource definisce il contratto per qualsiasi sorgente di dati.
type DataSource interface {
	Load() (*DataDataSource, error)
}

// --- Implementazioni dell'interfaccia DataSource ---

// CSVDataSource gestisce il caricamento dei dati da un file CSV.
type CSVDataSource struct {
	Path string
}

func (c *CSVDataSource) Load() (*DataDataSource, error) {
	file, err := os.Open(c.Path)
	if err != nil {
		return nil, fmt.Errorf("impossibile aprire il file CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere il file CSV: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("il file CSV è vuoto")
	}

	header := records[0]
	var data DataDataSource
	data.Header = header
	data.Records = make([][]string, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) != len(header) {
			return nil, fmt.Errorf("record con numero di colonne non corrispondente all'intestazione: %v", record)
		}
		data.Records = append(data.Records, record)
	}
	return &data, nil
}

// JSONDataSource gestisce il caricamento dei dati da un file JSON.
type JSONDataSource struct {
	Path string
}

func (j *JSONDataSource) Load() (*DataDataSource, error) {
	fileData, err := os.ReadFile(j.Path)
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere il file JSON: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(fileData, &data); err != nil {
		return nil, fmt.Errorf("impossibile analizzare il file JSON: %w", err)
	}
	return &data, nil
}

// APIDataSource gestisce il caricamento dei dati da un'API.
type APIDataSource struct {
	URL string
}

func (a *APIDataSource) Load() (*DataDataSource, error) {
	resp, err := http.Get(a.URL)
	if err != nil {
		return nil, fmt.Errorf("impossibile effettuare la richiesta all'API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("risposta API non riuscita, codice di stato: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere il corpo della risposta: %w", err)
	}

	var data DataDataSource
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("impossibile decodificare la risposta JSON dell'API: %w", err)
	}
	return &data, nil
}

// SystemMetricsDataSource gestisce il caricamento delle metriche di sistema.
type SystemMetricsDataSource struct{}

func (s *SystemMetricsDataSource) Load() (*DataDataSource, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("impossibile ottenere la memoria virtuale: %w", err)
	}

	c, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("impossibile ottenere l'utilizzo della CPU: %w", err)
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
		return nil, nil, fmt.Errorf("impossibile leggere il file di configurazione: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, nil, fmt.Errorf("impossibile analizzare il file di configurazione YAML: %w", err)
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
		return nil, nil, fmt.Errorf("tipo di sorgente dati non supportato: %s", config.Source.Type)
	}
    
    // Assegna i valori di ritorno a variabili e gestisci l'errore
	data, err := dataSource.Load()
	if err != nil {
		return nil, nil, err // Restituisci l'errore se il caricamento fallisce
	}

    // Se tutto va bene, restituisci i tre valori attesi
	return &config, data, nil
}