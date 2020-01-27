package service

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/websocket"
	"github.com/lancer-kit/armory/api/render"
	"github.com/lancer-kit/armory/log"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/info"
	"gitlab.inn4science.com/ctp/hermes/service/socket"
	"gitlab.inn4science.com/ctp/hermes/sessions"
)

func GetServer(logger *logrus.Entry, cfg config.Cfg, ctx context.Context, bus socket.EventStream) *api.Server {
	return api.NewServer(cfg.API, getRouter(ctx, logger, cfg, bus))
}

func getRouter(ctx context.Context, logger *logrus.Entry, cfg config.Cfg, bus socket.EventStream) http.Handler {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(log.NewRequestLogger(logger.Logger))

	if cfg.API.EnableCORS {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type",
				"X-CSRF-Token", "jwt", "X-UID", "Proxy-Authorization"},
			ExposedHeaders:   []string{"Link", "Content-Length"},
			AllowCredentials: true,
			MaxAge:           300, // Maximum value not ignored by any of major browsers
		})
		r.Use(corsHandler.Handler)
	}

	storage, err := sessions.NewStorage(cfg.Redis)
	if err != nil {
		logger.WithError(err).Fatal("unable to init redis")
	}

	h := handler{
		ctx:     ctx,
		log:     logger,
		bus:     bus,
		storage: storage,

		disableSessionCheck: cfg.Redis.DevMode,
	}

	r.Route("/_ws", func(r chi.Router) {
		r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
			render.Success(w, info.App)
		})

		r.Get("/subscribe", h.handleNewWS)
		r.Get("/gc", func(http.ResponseWriter, *http.Request) { runtime.GC() })

		// r.Get("/metrics", func(writer http.ResponseWriter, _ *http.Request) {
		// 	data, _ := socket.MetricsCollector.MarshalJSON()
		// 	writer.WriteHeader(200)
		// 	_, _ = writer.Write(data)
		// })

	})
	return r
}

type handler struct {
	disableSessionCheck bool
	storage             *sessions.Storage
	ctx                 context.Context
	log                 *logrus.Entry
	bus                 socket.EventStream
}

const (
	HeaderUUID  = "uuid"
	HeaderToken = "token"
)

func (h *handler) extractUUID(r *http.Request) (string, error) {
	sessionToken := r.URL.Query().Get(HeaderToken)
	if sessionToken == "" {
		uuid := r.URL.Query().Get(HeaderUUID)
		return uuid, nil
	}

	session, err := h.storage.GetSession(sessionToken)
	if err != nil {
		h.log.WithError(err).Error("failed to get session")
		return "", errors.New("failed to get session")
	}

	return session.UserID, nil
}
func (h handler) handleNewWS(w http.ResponseWriter, r *http.Request) {
	h.log.Debug("Socket open with method:", r.Method)

	sessionUID := time.Now().UnixNano()

	// MetricsCollector.Add("ws.client")
	userUID, err := h.extractUUID(r)
	if err != nil {
		render.ServerError(w)
		return
	}

	if userUID == "" || userUID == "undefined" {
		render.BadRequest(w, "uuid: must be not empty")
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  10 * 1024,
		WriteBufferSize: 10 * 1024,
		CheckOrigin:     func(*http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.WithError(err).Error("unable to upgrade http protocol")
		return
	}

	client := socket.NewSession(h.ctx, h.log, h.bus, conn, sessionUID, userUID)
	h.log.WithField("uuid", userUID).Debug("Open new client connection")
	h.bus <- &socket.Event{Kind: socket.EKNewSession, Session: client}
}
