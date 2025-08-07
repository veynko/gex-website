package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type AppConfig struct {
	NFQ_LOG_FILE    string `json:"nfq_log_file"`
	NFQ_CONFIG_FILE string `json:"nfq_config_file"`
	NFQ_RULES_DIR   string `json:"nfq_rules_dir"`
	NET_STATS_FILE  string `json:"net_stats_file"`
	SYS_STATS_FILE  string `json:"sys_stats_file"`
	LogLevel        string `json:"logLevel"`
	Interface       string `json:"interface"`
	ListenPort      string `json:"listenPort"`
}

var config *AppConfig

var prev_netstats NetworkStats

const CONFIG_FILE = "./config.json"

func loadConfig() (*AppConfig, error) {
	defaultConfig := &AppConfig{
		NFQ_LOG_FILE:    "/root/nfq/log.txt",
		NFQ_CONFIG_FILE: "/root/nfq/config.json",
		NFQ_RULES_DIR:   "/root/nfq/rules",
		NET_STATS_FILE:  "/tmp/nfq/nfq.json",
		SYS_STATS_FILE:  "/tmp/nfq/sys.json",
		Interface:       "lan0",
		LogLevel:        "info",
		ListenPort:      "8080",
	}

	if _, err := os.Stat(CONFIG_FILE); os.IsNotExist(err) {
		if err := saveConfig(defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create default config: %v", err)
		}
		return defaultConfig, nil
	}

	content, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg AppConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if cfg.NFQ_LOG_FILE == "" {
		cfg.NFQ_LOG_FILE = defaultConfig.NFQ_LOG_FILE
	}
	if cfg.NFQ_RULES_DIR == "" {
		cfg.NFQ_RULES_DIR = defaultConfig.NFQ_RULES_DIR
	}
	if cfg.NET_STATS_FILE == "" {
		cfg.NET_STATS_FILE = defaultConfig.NET_STATS_FILE
	}
	if cfg.SYS_STATS_FILE == "" {
		cfg.SYS_STATS_FILE = defaultConfig.SYS_STATS_FILE
	}
	if cfg.Interface == "" {
		cfg.Interface = defaultConfig.Interface
	}

	return &cfg, nil
}

func saveConfig(cfg *AppConfig) error {
	configData, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(CONFIG_FILE, configData, 0644)
}

type Dashboard struct {
	upgrader      websocket.Upgrader
	logClients    map[*websocket.Conn]bool
	logClientsMux sync.RWMutex
	lastLogSize   int64
}

type SystemStats struct {
	CPU       float64    `json:"cpu"`
	RAM       float64    `json:"ram"`
	RAMUsed   uint64     `json:"ramUsed"`
	RAMTotal  uint64     `json:"ramTotal"`
	Disk      float64    `json:"disk"`
	DiskUsed  uint64     `json:"diskUsed"`
	DiskTotal uint64     `json:"diskTotal"`
	Speed     SpeedStats `json:"speed"`
	Timestamp int64      `json:"timestamp"`
}

type NetworkStats struct {
	BytesRecv uint64 `json:"bytesRecv"`
	BytesSent uint64 `json:"bytesSent"`
}

type SpeedStats struct {
	Download uint64 `json:"download"`
	Upload   uint64 `json:"upload"`
}

type PacketStats struct {
	Total   uint64 `json:"total"`
	Passed  uint64 `json:"passed"`
	Blocked uint64 `json:"blocked"`
}

type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Action      string `json:"action"`
	Protocol    string `json:"protocol"`
	SourceIP    string `json:"sourceIP"`
	DestIP      string `json:"destIP"`
	SourcePort  int    `json:"sourcePort"`
	DestPort    int    `json:"destPort"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

func NewDashboard() *Dashboard {
	d := &Dashboard{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		logClients: make(map[*websocket.Conn]bool),
	}

	d.initLogWatcher()

	return d
}

func main() {
	var err error
	config, err = loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	netStats, err := net.IOCounters(true)
	if err != nil {
		log.Fatal("Failed to get network stats:", err)
	}

	var networkStats NetworkStats
	for _, stat := range netStats {
		if stat.Name == config.Interface {
			networkStats = NetworkStats{
				BytesRecv: stat.BytesRecv,
				BytesSent: stat.BytesSent,
			}
			break
		}
	}

	prev_netstats = networkStats

	os.MkdirAll(filepath.Dir(config.NFQ_RULES_DIR), 0755)
	os.MkdirAll(config.NFQ_RULES_DIR, 0755)

	createDefaultFiles()

	dashboard := NewDashboard()

	r := mux.NewRouter()

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// HTML pages
	r.HandleFunc("/", dashboard.mainHandler)
	r.HandleFunc("/config", dashboard.configHandler)
	r.HandleFunc("/logs", dashboard.logsHandler)
	r.HandleFunc("/rules", dashboard.rulesHandler)

	// API
	r.HandleFunc("/api/stats", dashboard.statsHandler)
	r.HandleFunc("/api/logs", dashboard.getLogsHandler)
	r.HandleFunc("/api/config", dashboard.configAPIHandler).Methods("GET", "POST")
	r.HandleFunc("/api/rules/files", dashboard.ruleFilesHandler).Methods("GET")
	r.HandleFunc("/api/rules/raw/{filename}", dashboard.rawRuleHandler).Methods("GET", "POST", "PUT", "DELETE")
	r.HandleFunc("/api/rules", dashboard.rulesAPIHandler).Methods("GET", "POST")
	r.HandleFunc("/api/rules/{id}", dashboard.ruleAPIHandler).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/api/packet-stats", dashboard.packetStatsHandler)
	r.HandleFunc("/api/restart/{service}", dashboard.restartServiceHandler).Methods("POST")

	// WebSocket
	r.HandleFunc("/ws/stats", dashboard.wsStatsHandler)
	r.HandleFunc("/ws/logs", dashboard.wsLogsHandler)

	fmt.Println("Dashboard запущен на http://localhost:8080")
	fmt.Println("Веб-интерфейс: http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func createDefaultFiles() {
	if _, err := os.Stat(config.NFQ_LOG_FILE); os.IsNotExist(err) {
		os.WriteFile(config.NFQ_LOG_FILE, []byte(""), 0644)
	}

	if _, err := os.Stat(config.NET_STATS_FILE); os.IsNotExist(err) {
		defaultNetStats := map[string]interface{}{
			"total":   1488,
			"passed":  488,
			"blocked": 1000,
		}
		data, _ := json.MarshalIndent(defaultNetStats, "", "    ")
		os.WriteFile(config.NET_STATS_FILE, data, 0644)
	}

	if _, err := os.Stat(config.SYS_STATS_FILE); os.IsNotExist(err) {
		defaultSysStats := map[string]interface{}{
			"cpu":       0.0,
			"ram":       0.0,
			"disk":      0.0,
			"timestamp": time.Now().Unix(),
		}
		data, _ := json.MarshalIndent(defaultSysStats, "", "    ")
		os.WriteFile(config.SYS_STATS_FILE, data, 0644)
	}
}

func (d *Dashboard) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := d.getSystemStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) getSystemStats() (*SystemStats, error) {
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	diskStat, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	netStats, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	var networkStats NetworkStats
	for _, stat := range netStats {
		if stat.Name == config.Interface {
			networkStats = NetworkStats{
				BytesRecv: stat.BytesRecv,
				BytesSent: stat.BytesSent,
			}
			break
		}
	}

	var speedStats = SpeedStats{
		Download: networkStats.BytesRecv - prev_netstats.BytesRecv,
		Upload:   networkStats.BytesSent - prev_netstats.BytesSent,
	}

	stats := &SystemStats{
		CPU:       cpuPercent[0],
		RAM:       memory.UsedPercent,
		RAMUsed:   memory.Used,
		RAMTotal:  memory.Total,
		Disk:      diskStat.UsedPercent,
		DiskUsed:  diskStat.Used,
		DiskTotal: diskStat.Total,
		Speed:     speedStats,
		Timestamp: time.Now().Unix(),
	}

	prev_netstats = networkStats

	return stats, nil
}

func (d *Dashboard) packetStatsHandler(w http.ResponseWriter, r *http.Request) {
	statsFile := config.NET_STATS_FILE

	if content, err := os.ReadFile(statsFile); err == nil {
		var fileStats map[string]interface{}
		if json.Unmarshal(content, &fileStats) == nil {
			stats := PacketStats{}
			if total, ok := fileStats["total"].(float64); ok {
				stats.Total = uint64(total)
			}
			if passed, ok := fileStats["passed"].(float64); ok {
				stats.Passed = uint64(passed)
			}
			if blocked, ok := fileStats["blocked"].(float64); ok {
				stats.Blocked = uint64(blocked)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
			return
		}
	}

	stats := PacketStats{
		Total:   1,
		Passed:  0,
		Blocked: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) initLogWatcher() {
	if info, err := os.Stat(config.NFQ_LOG_FILE); err == nil {
		d.lastLogSize = info.Size()
	}

	go d.watchLogFile()
}

func (d *Dashboard) watchLogFile() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if info, err := os.Stat(config.NFQ_LOG_FILE); err == nil {
			currentSize := info.Size()
			if currentSize > d.lastLogSize {
				d.readNewLogLines()
				d.lastLogSize = currentSize
			}
		}
	}
}

func (d *Dashboard) readNewLogLines() {
	file, err := os.Open(config.NFQ_LOG_FILE)
	if err != nil {
		return
	}
	defer file.Close()

	if _, err := file.Seek(d.lastLogSize, 0); err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	var newLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			newLines = append(newLines, line)
		}
	}

	if len(newLines) > 0 {
		d.broadcastLogLines(newLines)
	}
}

func (d *Dashboard) broadcastLogLines(lines []string) {
	d.logClientsMux.RLock()
	defer d.logClientsMux.RUnlock()

	message := map[string]interface{}{
		"type":      "logs",
		"lines":     lines,
		"timestamp": time.Now().Unix(),
	}

	for client := range d.logClients {
		if err := client.WriteJSON(message); err != nil {
			delete(d.logClients, client)
			client.Close()
		}
	}
}

func (d *Dashboard) wsLogsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	d.logClientsMux.Lock()
	d.logClients[conn] = true
	d.logClientsMux.Unlock()

	d.sendRecentLogs(conn)

	defer func() {
		d.logClientsMux.Lock()
		delete(d.logClients, conn)
		d.logClientsMux.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (d *Dashboard) sendRecentLogs(conn *websocket.Conn) {
	content, err := os.ReadFile(config.NFQ_LOG_FILE)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	startIndex := 0
	if len(lines) > 50 {
		startIndex = len(lines) - 50
	}

	var recentLines []string
	for i := startIndex; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			recentLines = append(recentLines, lines[i])
		}
	}

	if len(recentLines) > 0 {
		message := map[string]interface{}{
			"type":      "initial_logs",
			"lines":     recentLines,
			"timestamp": time.Now().Unix(),
		}

		conn.WriteJSON(message)
	}
}

func (d *Dashboard) restartServiceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	service := vars["service"]

	var cmd *exec.Cmd
	switch service {
	case "web":
		cmd = exec.Command("sudo", "systemctl", "restart", "gex-web")
	case "nfq":
		cmd = exec.Command("sudo", "systemctl", "restart", "gex-nfq")
	default:
		http.Error(w, "Неизвестная служба", http.StatusBadRequest)
		return
	}

	err := cmd.Run()
	response := map[string]interface{}{
		"success": err == nil,
		"service": service,
	}

	if err != nil {
		response["error"] = err.Error()
		response["message"] = "Ошибка перезапуска службы " + service
	} else {
		response["message"] = "Служба " + service + " успешно перезапущена"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
