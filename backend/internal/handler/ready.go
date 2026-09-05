// Package handler — публичные HTTP-обработчики Go-монолита. Регистрация
// маршрутов выполняется через Register; пока это только собственная
// готовность Go. Маршруты пользователя добавляет B3 в этом же пакете.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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

// ReadinessCheck is supplied by the composition root for dependencies such as
// PostgreSQL. The handler owns only the short request timeout.
type ReadinessCheck func(context.Context) error

// Register подключает публичные маршруты к mux.
func Register(mux *http.ServeMux, checks ...ReadinessCheck) {
	var check ReadinessCheck
	if len(checks) > 0 {
		check = checks[0]
	}
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		handleReady(w, r, check)
	})
}

func handleReady(w http.ResponseWriter, r *http.Request, check ReadinessCheck) {
	if check != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := check(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "service dependencies are unavailable", true)
			return
		}
	}
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
