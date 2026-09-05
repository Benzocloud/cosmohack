package handler

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/Benzocloud/cosmohack/backend/internal/domain"
	"github.com/Benzocloud/cosmohack/backend/internal/service/store"
)

// ErrQueueFull — Enqueue отклонил постановку (8 чужих ожидающих).
var ErrQueueFull = errors.New("queue full")

// Queue ставит job в очередь B4. Не ходит в store.
type Queue interface {
	Enqueue(ctx context.Context, jobID string) error
}

// Canceller — необязательная отмена ожидания при DELETE.
type Canceller interface {
	Cancel(jobID string)
}

// Contour — элемент каталога поиска.
type Contour struct {
	ID       string        `json:"id"`
	Geometry store.Polygon `json:"geometry"`
	Source   ContourSource `json:"source"`
}

type ContourSource struct {
	Provider    string `json:"provider"`
	Attribution string `json:"attribution"`
}

// ContourFinder — поиск B1 по bbox.
type ContourFinder interface {
	Find(ctx context.Context, minLon, minLat, maxLon, maxLat float64) ([]Contour, error)
}

// Storage is the handler's domain persistence port. Implementations own the
// database or legacy storage mapping; HTTP handlers only exchange domain values.
type Storage interface {
	CreateArea(context.Context, domain.Area) error
	UpdateArea(context.Context, domain.Area) error
	GetArea(context.Context, string) (domain.Area, error)
	ListAreas(context.Context) ([]domain.Area, error)
	DeleteArea(context.Context, string) ([]string, error)
	GetJob(context.Context, string) (domain.Job, error)
	ListJobsByArea(context.Context, string) ([]domain.Job, error)
	PutJobQueued(context.Context, domain.Job) error
	DeleteJob(context.Context, string) error
	GetResult(context.Context, string, string) (domain.AnalysisRecord, error)
}

type handler struct {
	storage  Storage
	contours ContourFinder
	queue    Queue
	limits   Limits
	gate     sync.Mutex
}

// NewMux собирает публичные маршруты без Listen. Возвращает *http.ServeMux,
// чтобы app добавлял /readyz тем же mux. Лимиты площади/вершин: ноль значит
// «не проверять».
func NewMux(st *store.Store, contours ContourFinder, queue Queue, lim Limits) *http.ServeMux {
	return NewMuxWithStorage(legacyStorage{store: st}, contours, queue, lim)
}

// NewMuxWithStorage builds routes over a domain persistence implementation.
func NewMuxWithStorage(storage Storage, contours ContourFinder, queue Queue, lim Limits) *http.ServeMux {
	h := &handler{storage: storage, contours: contours, queue: queue, limits: lim}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/areas", h.listAreas)
	mux.HandleFunc("POST /api/areas", h.createArea)
	mux.HandleFunc("DELETE /api/areas/{id}", h.deleteArea)
	mux.HandleFunc("GET /api/areas/{id}/series", h.getSeries)
	mux.HandleFunc("GET /api/areas/{id}/events", h.getEvents)
	mux.HandleFunc("POST /api/areas/{id}/analyses", h.postAnalyses)
	mux.HandleFunc("GET /api/jobs/{id}", h.getJob)
	mux.HandleFunc("GET /api/regions/contours", h.getContours)
	return mux
}
