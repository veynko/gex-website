package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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

const (
	APP_DIR      = "/opt/gex"
	NFQ_DIR      = "/opt/nfq"
	NFQ_LOG_FILE = "./test_log.txt"
	CONFIG_FILE  = APP_DIR + "/config.json"
	RULES_DIR    = APP_DIR + "/rules"
)

type Dashboard struct {
	upgrader      websocket.Upgrader
	logClients    map[*websocket.Conn]bool
	logClientsMux sync.RWMutex
	lastLogSize   int64
}

type SystemStats struct {
	CPU       float64      `json:"cpu"`
	RAM       float64      `json:"ram"`
	RAMUsed   uint64       `json:"ramUsed"`
	RAMTotal  uint64       `json:"ramTotal"`
	Disk      float64      `json:"disk"`
	DiskUsed  uint64       `json:"diskUsed"`
	DiskTotal uint64       `json:"diskTotal"`
	Network   NetworkStats `json:"network"`
	Timestamp int64        `json:"timestamp"`
}

type NetworkStats struct {
	BytesRecv   uint64 `json:"bytesRecv"`
	BytesSent   uint64 `json:"bytesSent"`
	PacketsRecv uint64 `json:"packetsRecv"`
	PacketsSent uint64 `json:"packetsSent"`
}

type PacketStats struct {
	Total   uint64 `json:"total"`
	Passed  uint64 `json:"passed"`
	Blocked uint64 `json:"blocked"`
}

type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Action      string `json:"action"` // allow, block
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
	os.MkdirAll(APP_DIR, 0755)
	os.MkdirAll(RULES_DIR, 0755)
	createDefaultFiles()

	dashboard := NewDashboard()

	r := mux.NewRouter()

	// Static
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
	if _, err := os.Stat(CONFIG_FILE); os.IsNotExist(err) {
		defaultConfig := map[string]interface{}{
			"interface":  "eth0",
			"mode":       "bridge",
			"logLevel":   "info",
			"bufferSize": 4096,
		}
		configData, _ := json.MarshalIndent(defaultConfig, "", "    ")
		os.WriteFile(CONFIG_FILE, configData, 0644)
	}

	if _, err := os.Stat(NFQ_LOG_FILE); os.IsNotExist(err) {
		os.WriteFile(NFQ_LOG_FILE, []byte(""), 0644)
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
	// CPU статистика
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	// RAM статистика
	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	// Диск статистика
	diskStat, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	// Сетевая статистика
	netStats, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}

	var networkStats NetworkStats
	if len(netStats) > 0 {
		networkStats = NetworkStats{
			BytesRecv:   netStats[0].BytesRecv,
			BytesSent:   netStats[0].BytesSent,
			PacketsRecv: netStats[0].PacketsRecv,
			PacketsSent: netStats[0].PacketsSent,
		}
	}

	stats := &SystemStats{
		CPU:       cpuPercent[0],
		RAM:       memory.UsedPercent,
		RAMUsed:   memory.Used,
		RAMTotal:  memory.Total,
		Disk:      diskStat.UsedPercent,
		DiskUsed:  diskStat.Used,
		DiskTotal: diskStat.Total,
		Network:   networkStats,
		Timestamp: time.Now().Unix(),
	}

	return stats, nil
}

func (d *Dashboard) packetStatsHandler(w http.ResponseWriter, r *http.Request) {
	statsFile := APP_DIR + "/packet_stats.json"

	// Пытаемся прочитать статистику из файла
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

	// Fallback: имитация статистики пакетов
	stats := PacketStats{
		Total:   12345,
		Passed:  10000,
		Blocked: 2345,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// initLogWatcher инициализирует watcher для файла логов
func (d *Dashboard) initLogWatcher() {
	// Получаем текущий размер файла
	if info, err := os.Stat(NFQ_LOG_FILE); err == nil {
		d.lastLogSize = info.Size()
	}

	// Запускаем горутину для периодической проверки изменений файла
	go d.watchLogFile()
}

// watchLogFile отслеживает изменения в файле логов
func (d *Dashboard) watchLogFile() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		if info, err := os.Stat(NFQ_LOG_FILE); err == nil {
			currentSize := info.Size()
			if currentSize > d.lastLogSize {
				// Файл увеличился, читаем новые строки
				d.readNewLogLines()
				d.lastLogSize = currentSize
			}
		}
	}
}

// readNewLogLines читает новые строки из файла логов
func (d *Dashboard) readNewLogLines() {
	file, err := os.Open(NFQ_LOG_FILE)
	if err != nil {
		return
	}
	defer file.Close()

	// Переходим к позиции последнего прочитанного байта
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

// broadcastLogLines отправляет новые строки логов всем подключенным клиентам
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
			// Клиент отключился, удаляем его
			delete(d.logClients, client)
			client.Close()
		}
	}
}

// wsLogsHandler обрабатывает WebSocket соединения для real-time логов
func (d *Dashboard) wsLogsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Добавляем клиента в список
	d.logClientsMux.Lock()
	d.logClients[conn] = true
	d.logClientsMux.Unlock()

	// Отправляем последние строки логов при подключении
	d.sendRecentLogs(conn)

	// Обрабатываем сообщения от клиента (в основном для поддержания соединения)
	defer func() {
		d.logClientsMux.Lock()
		delete(d.logClients, conn)
		d.logClientsMux.Unlock()
		conn.Close()
	}()

	for {
		// Читаем сообщения от клиента для поддержания соединения
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// sendRecentLogs отправляет последние строки логов новому клиенту
func (d *Dashboard) sendRecentLogs(conn *websocket.Conn) {
	content, err := os.ReadFile(NFQ_LOG_FILE)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	// Отправляем последние 50 строк
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
		// Перезапуск веб-службы
		cmd = exec.Command("systemctl", "restart", "gex-web")
	case "nfq":
		// Перезапуск NFQ службы
		cmd = exec.Command("systemctl", "restart", "gex-nfq")
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
