# Datacmd: The Ultimate Terminal Dashboard Generator 🚀  

Welcome to **Datacmd**, the most intuitive and powerful terminal-based dashboard generator you'll ever need! Inspired by the groundbreaking **Datastripes**, this project takes simplicity, flexibility, and data visualization to a whole new level.  

## 🌟 Why Datacmd?  

Imagine transforming your raw data into stunning, interactive dashboards directly in your terminal. No complex setups, no bloated GUIs—just pure, efficient, and beautiful dashboards that you can generate in seconds.  

### Key Features:  
- **Dynamic Dashboard Generation**: Automatically generate dashboards from CSV, JSON, APIs, or even system metrics.  
- **Wide Range of Widgets**: From tables to pie charts, radar charts, gauges, and more—visualize your data the way you want.  
- **Terminal-First Design**: Built with terminal libraries like `termdash`, Datacmd ensures a seamless and responsive experience.  
- **Customizable Layouts**: Dynamically adapt your dashboard layout to fit your data and preferences.  
- **Lightning-Fast Setup**: Generate dashboards with a single command.  

## 🛠️ How It Works  

1. **Provide Your Data**:  
    Datacmd supports multiple data sources:  
    - CSV files  
    - JSON files  
    - APIs  
    - System metrics (CPU, memory, etc.)  

2. **Generate or Configure**:  
    - Use the `--generate` flag to automatically create a dashboard configuration based on your data.  
    - Or, customize your dashboard using a simple YAML configuration file.  

3. **Run and Enjoy**:  
    Watch your data come to life in a beautifully crafted terminal dashboard.  

## 🚀 Quick Start  

1. Clone the repository:  
    ```bash  
    git clone https://github.com/your-repo/datacmd.git  
    cd datacmd  
    ```  

2. Install dependencies:  
    ```bash  
    go mod tidy  
    ```  

3. Generate a dashboard:  
    ```bash  
    go run main.go --generate --source=path/to/your/data.csv  
    ```  

4. Run the dashboard:  
    ```bash  
    go run main.go --config=config.yml  
    ```  

## ✨ Example  

### Input: `sample.csv`  
```csv  
label,value,category  
A,75,Category1  
B,82,Category2  
C,91,Category1  
```  

### Output:  
A terminal dashboard with:  
- A table displaying the data.  
- A pie chart visualizing the values.  
- A radar chart comparing categories.  

## 🌌 The Legacy of Datastripes  

Datacmd is built on the shoulders of **Datastripes**, a revolutionary project that redefined how we visualize data in the terminal. While Datastripes focused on horizontal data visualization, Datacmd expands the horizon with dynamic dashboards and interactive widgets.  

## 🤝 Contributing  

We welcome contributions! Whether it's fixing a bug, adding a feature, or improving documentation, your help is appreciated.  

## 📜 License  

This project is licensed under the [Apache License 2.0](LICENSE).  

---

**Datacmd**: Where your data meets the terminal magic. Try it today and experience the future of terminal dashboards!  