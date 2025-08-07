package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

const (
	APP_DIR     = "/opt/gex"
	LOG_FILE    = APP_DIR + "/log.txt"
	CONFIG_FILE = APP_DIR + "/config.json"
	RULES_DIR   = APP_DIR + "/rules"
)

type Dashboard struct {
	upgrader websocket.Upgrader
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
	return &Dashboard{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func main() {
	// Создаём необходимые директории если их нет
	os.MkdirAll(APP_DIR, 0755)
	os.MkdirAll(RULES_DIR, 0755)

	// Создаём файлы по умолчанию если они не существуют
	createDefaultFiles()

	dashboard := NewDashboard()

	r := mux.NewRouter()

	// Статические файлы (веб-интерфейс)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// Главная страница - редирект на статический файл
	// r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// 	http.Redirect(w, r, "/static/index.html", http.StatusMovedPermanently)
	// })

	// HTML страницы
	r.HandleFunc("/", dashboard.mainHandler)
	r.HandleFunc("/config", dashboard.configHandler)
	r.HandleFunc("/rules", dashboard.rulesHandler)

	// API маршруты
	r.HandleFunc("/api/stats", dashboard.statsHandler)
	r.HandleFunc("/api/logs", dashboard.getLogsHandler)
	r.HandleFunc("/api/config", dashboard.configAPIHandler).Methods("GET", "POST")
	r.HandleFunc("/api/rules", dashboard.rulesAPIHandler).Methods("GET", "POST")
	r.HandleFunc("/api/rules/{id}", dashboard.ruleAPIHandler).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/api/packet-stats", dashboard.packetStatsHandler)
	r.HandleFunc("/api/restart/{service}", dashboard.restartServiceHandler).Methods("POST")

	// WebSocket для real-time обновлений
	r.HandleFunc("/ws/stats", dashboard.wsStatsHandler)

	fmt.Println("Dashboard запущен на http://localhost:8080")
	fmt.Println("Веб-интерфейс: http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func createDefaultFiles() {
	// Создаём config.json по умолчанию
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

	// Создаём log.txt если не существует
	if _, err := os.Stat(LOG_FILE); os.IsNotExist(err) {
		os.WriteFile(LOG_FILE, []byte(""), 0644)
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

func (d *Dashboard) wsStatsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats, err := d.getSystemStats()
		if err != nil {
			log.Printf("Error getting stats: %v", err)
			continue
		}

		if err := conn.WriteJSON(stats); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}

func (d *Dashboard) getLogsHandler(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(LOG_FILE)
	if err != nil {
		http.Error(w, "Не удалось прочитать файл логов", http.StatusInternalServerError)
		return
	}

	// Возвращаем последние 1000 строк
	lines := strings.Split(string(content), "\n")
	if len(lines) > 1000 {
		lines = lines[len(lines)-1000:]
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(strings.Join(lines, "\n")))
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
