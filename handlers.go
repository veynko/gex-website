package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func (d *Dashboard) mainHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
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

func (d *Dashboard) configHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/config.html")
}

func (d *Dashboard) logsHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/logs.html")
}

func (d *Dashboard) getLogsHandler(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile(config.NFQ_LOG_FILE)
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

func (d *Dashboard) configAPIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		content, err := os.ReadFile("./config.json")
		if err != nil {
			http.Error(w, "Не удалось прочитать конфигурацию", http.StatusInternalServerError)
			return
		}

		var config map[string]interface{}
		if err := json.Unmarshal(content, &config); err != nil {
			http.Error(w, "Некорректный JSON в конфигурации", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)

	case "POST":
		var config map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Некорректный JSON", http.StatusBadRequest)
			return
		}

		configData, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			http.Error(w, "Ошибка сериализации JSON", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile("./config.json", configData, 0644); err != nil {
			http.Error(w, "Не удалось сохранить конфигурацию", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Конфигурация сохранена",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (d *Dashboard) rulesHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/rules.html")
}

func (d *Dashboard) rulesAPIHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rules, err := d.loadAllRules()
		if err != nil {
			http.Error(w, "Ошибка загрузки правил", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)

	case "POST":
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Некорректные данные", http.StatusBadRequest)
			return
		}

		if rule.ID == "" {
			rule.ID = fmt.Sprintf("rule_%d", time.Now().Unix())
		}

		if err := d.saveRule(&rule); err != nil {
			http.Error(w, "Ошибка сохранения правила", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Правило создано",
			"rule":    rule,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (d *Dashboard) ruleAPIHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ruleID := vars["id"]

	switch r.Method {
	case "GET":
		rule, err := d.loadRule(ruleID)
		if err != nil {
			http.Error(w, "Правило не найдено", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)

	case "PUT":
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Некорректные данные", http.StatusBadRequest)
			return
		}

		rule.ID = ruleID
		if err := d.saveRule(&rule); err != nil {
			http.Error(w, "Ошибка сохранения правила", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Правило обновлено",
			"rule":    rule,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case "DELETE":
		if err := d.deleteRule(ruleID); err != nil {
			http.Error(w, "Ошибка удаления правила", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Правило удалено",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func (d *Dashboard) loadAllRules() ([]Rule, error) {
	var rules []Rule

	// Читаем все файлы .json из директории rules
	files, err := os.ReadDir(config.NFQ_RULES_DIR)
	if err != nil {
		return rules, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			rule, err := d.loadRule(strings.TrimSuffix(file.Name(), ".json"))
			if err != nil {
				continue // Пропускаем файлы с ошибками
			}
			rules = append(rules, *rule)
		}
	}

	// Сортируем по ID
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].ID < rules[j].ID
	})

	return rules, nil
}

func (d *Dashboard) loadRule(ruleID string) (*Rule, error) {
	filePath := filepath.Join(config.NFQ_RULES_DIR, ruleID+".json")

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var rule Rule
	if err := json.Unmarshal(content, &rule); err != nil {
		return nil, err
	}

	return &rule, nil
}

func (d *Dashboard) saveRule(rule *Rule) error {
	// Создаём директорию если не существует
	if err := os.MkdirAll(config.NFQ_RULES_DIR, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(config.NFQ_RULES_DIR, rule.ID+".json")

	ruleData, err := json.MarshalIndent(rule, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, ruleData, 0644)
}

func (d *Dashboard) deleteRule(ruleID string) error {
	filePath := filepath.Join(config.NFQ_RULES_DIR, ruleID+".json")
	return os.Remove(filePath)
}
