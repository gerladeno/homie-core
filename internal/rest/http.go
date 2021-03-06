package rest

import (
	"compress/flate"
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gerladeno/homie-core/pkg/chat"

	"github.com/gerladeno/homie-core/internal/models"
	"github.com/gerladeno/homie-core/pkg/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/sirupsen/logrus"
)

type Service interface {
	SaveConfig(ctx context.Context, config *models.Config) error
	GetConfig(ctx context.Context, uuid string) (*models.Config, error)
	GetRegions(ctx context.Context) ([]*models.Region, error)
	Like(ctx context.Context, uuid, targetUUID string, super bool) error
	Dislike(ctx context.Context, uuid, targetUUID string) error
	ListLikedProfiles(ctx context.Context, uuid string, limit, offset int64) ([]*models.Profile, error)
	ListDislikedProfiles(ctx context.Context, uuid string, limit, offset int64) ([]*models.Profile, error)
	GetMatches(ctx context.Context, uuid string, count int64) ([]*models.Profile, error)
	GetDialog(ctx context.Context, client, target string) *chat.Hub
	GetAllChats(ctx context.Context, uuid string) ([]*models.Profile, error)
}

const gitURL = "https://github.com/gerladeno/homie-core"

func NewRouter(log *logrus.Logger, service Service, key *rsa.PublicKey, host, version string) chi.Router {
	handler := newHandler(log, service, key)
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(cors.AllowAll().Handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.NewCompressor(flate.DefaultCompression).Handler)
	r.NotFound(notFoundHandler)
	r.Get("/ping", pingHandler)
	r.Get("/version", versionHandler(version))
	r.Group(func(r chi.Router) {
		r.Use(metrics.NewPromMiddleware(host))
		r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log, NoColor: true}))
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(middleware.Throttle(100))
		r.Route("/static", func(r chi.Router) {
			r.Get("/regions", handler.getRegions)
		})
		r.Route("/public", func(r chi.Router) {
			r.Use(handler.jwtAuth)
			r.Route("/v1", func(r chi.Router) {
				r.Group(func(r chi.Router) {
					r.Get("/config", handler.getConfig)
					r.Put("/config", handler.saveConfig)
					r.Get("/matches", handler.getMatches)
					r.Get("/like/{uuid}", handler.like)
					r.Get("/dislike/{uuid}", handler.dislike)
					r.Get("/liked", handler.listLiked)
					r.Get("/disliked", handler.listDisliked)
					r.Get("/chats", handler.getAllChats)
					r.HandleFunc("/chat/{uuid}", handler.chatHandler)
				})
			})
		})
		r.Route("/private", func(r chi.Router) {
		})
	})
	return r
}

func notFoundHandler(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "404 page not found. Check docs: "+gitURL, http.StatusNotFound)
}

func pingHandler(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, "pong")
}

func versionHandler(version string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(w, version)
	}
}

func writeResponse(w http.ResponseWriter, data interface{}) {
	response := JSONResponse{Data: data}
	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(response) //nolint:errchkjson
}

func writeErrResponse(w http.ResponseWriter, message string, status int) {
	response := JSONResponse{Data: []int{}, Error: &message, Code: &status}
	w.WriteHeader(status)
	w.Header().Set("Content-type", "application/json")
	_ = json.NewEncoder(w).Encode(response) //nolint:errchkjson
}

type JSONResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
	Error *string     `json:"error,omitempty"`
	Code  *int        `json:"code,omitempty"`
}

type Meta struct {
	Count int `json:"count"`
}
