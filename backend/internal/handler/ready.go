// Package handler — публичные HTTP-обработчики Go-монолита. Регистрация
// маршрутов выполняется через Register; пока это только собственная
// готовность Go. Маршруты пользователя добавляет B3 в этом же пакете.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
)

// readyStatus — значение статуса собственной готовности Go. Форма тела
// зеркалит readiness-контракт, но словарь готовности Go не зависит от
// словаря ML-контракта (domain.MLReadyStatus — только для пакета ml).
const readyStatus = "ready"

// readyResponse — собственная готовность Go по контракту полей /readyz.
// Публичный /readyz не вызывает ML: готовность ML проверяется отдельно
// через service/ml.
type readyResponse struct {
	Status          string   `json:"status"`
	SchemaVersion   string   `json:"schema_version"`
	FeatureProfiles []string `json:"feature_profiles"`
}

// Register подключает публичные маршруты к mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /readyz", handleReady)
}

func handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(readyResponse{
		Status:          readyStatus,
		SchemaVersion:   domain.SchemaVersionV1,
		FeatureProfiles: []string{domain.FeatureProfileNDVIWeatherV1},
	}); err != nil {
		slog.Error("readyz response encode failed", "error", err)
	}
}
