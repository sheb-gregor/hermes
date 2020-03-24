package service

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/websocket"
	"github.com/lancer-kit/armory/api/httpx"
	"github.com/lancer-kit/armory/api/render"
	"github.com/lancer-kit/armory/log"
	"github.com/lancer-kit/noble"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/pkg/errors"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"gitlab.inn4science.com/ctp/hermes/config"
	"gitlab.inn4science.com/ctp/hermes/info"
	"gitlab.inn4science.com/ctp/hermes/models"
	"gitlab.inn4science.com/ctp/hermes/service/socket"
)

func GetServer(logger *logrus.Entry, cfg config.Cfg, ctx context.Context, hubCom socket.HubCommunicator) *api.Server {
	return api.NewServer(cfg.API, getRouter(ctx, logger, cfg, hubCom))
}

func getRouter(ctx context.Context, logger *logrus.Entry, cfg config.Cfg, hubCom socket.HubCommunicator) http.Handler {
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

	h := handler{
		ctx:         ctx,
		log:         logger,
		hubCom:      hubCom,
		enableAuth:  cfg.EnableAuth,
		authCfg:     cfg.AuthProviders,
		serviceKeys: cfg.AuthorizedServices,
	}

	r.Route("/_ws", func(r chi.Router) {
		r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
			render.Success(w, info.App)
		})

		r.Get("/subscribe", h.handleNewWS)
		r.Get("/gc", func(http.ResponseWriter, *http.Request) { runtime.GC() })
		r.Get("/sessions/authorized", h.handleActiveSession)

		// r.Get("/metrics", func(writer http.ResponseWriter, _ *http.Request) {
		// 	data, _ := socket.MetricsCollector.MarshalJSON()
		// 	writer.WriteHeader(200)
		// 	_, _ = writer.Write(data)
		// })

	})
	return r
}

type handler struct {
	ctx    context.Context
	log    *logrus.Entry
	hubCom socket.HubCommunicator

	enableAuth  bool
	authCfg     map[string]config.AuthProvider
	serviceKeys map[string]noble.Secret
}

func (h handler) handleActiveSession(w http.ResponseWriter, r *http.Request) {
	service := r.Header.Get(models.APIServiceHeader)
	securityKey := r.Header.Get(models.APIKeyHeader)

	if h.enableAuth {
		key, ok := h.serviceKeys[service]
		if !ok || securityKey == "" || securityKey != key.Get() {
			render.Forbidden(w, "auth data invalid")
			return
		}
	}

	sessions := h.hubCom.GetSessions()
	render.Success(w, sessions)
}

func (h handler) handleNewWS(w http.ResponseWriter, r *http.Request) {
	logger := log.IncludeRequest(h.log, r)

	authInfo := models.SessionInfo{
		ID:        time.Now().UnixNano(),
		IP:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
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

	client := socket.NewSession(h.ctx, h.log, h.hubCom.EventBus, conn, authInfo, h.authProvider)

	logger.Debug("Open new client connection")
	h.hubCom.EventBus <- &socket.Event{Kind: socket.EKNewSession, Session: client}
}

func (h *handler) authProvider(req models.AuthRequest) (*models.AuthResponse, int, error) {
	if !h.enableAuth {
		return &models.AuthResponse{UserID: req.Token, TTL: -1}, http.StatusOK, nil
	}

	provider, ok := h.authCfg[req.Origin]
	if !ok {
		return nil, http.StatusNotAcceptable, errors.New("unknown origin")
	}

	if !provider.CheckRole(req.Role) {
		return nil, http.StatusNotAcceptable, errors.New("role not allowed")
	}

	resp, err := httpx.PostJSON(provider.URL.Str, req, map[string]string{provider.Header: provider.AccessKey.Get()})
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to check authorization")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errors.New("forbidden")
	}

	authInfo := new(models.AuthResponse)
	err = httpx.ParseJSONResult(resp, authInfo)
	if resp.StatusCode != http.StatusOK {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to parse auth response")
	}

	return authInfo, http.StatusOK, nil
}
