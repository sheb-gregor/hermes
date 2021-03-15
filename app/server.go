package app

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"hermes/app/ws"
	"hermes/config"
	"hermes/log"
	"hermes/metrics"
	"hermes/models"
	"hermes/web"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"github.com/lancer-kit/armory/api/httpx"
	"github.com/lancer-kit/armory/api/render"
	"github.com/lancer-kit/noble"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func GetServer(logger zerolog.Logger, cfg config.Cfg, ctx context.Context, hubCom ws.HubCommunicator) *api.Server {
	return api.NewServer(cfg.API, getRouter(ctx, logger, cfg, hubCom))
}

func getRouter(ctx context.Context, logger zerolog.Logger, cfg config.Cfg, hubCom ws.HubCommunicator) http.Handler {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(log.LoggerMiddleware(&logger))

	if cfg.API.EnableCORS {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type",
				"X-CSRF-Token", "X-UID", "Proxy-Authorization"},
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
		r.Get("/gc", func(http.ResponseWriter, *http.Request) { runtime.GC() })

		if cfg.EnableUI {
			r.Get("/client-ui", h.renderWebPage)
		}

		r.Get("/info", func(w http.ResponseWriter, r *http.Request) { render.Success(w, config.App) })
		r.Get("/sessions/authorized", h.handleActiveSession)
		r.Get("/subscribe", h.handleNewWS)

	})
	r.Mount("/", metrics.GetMonitoringMux(cfg.Monitoring))
	return r
}

type handler struct {
	ctx    context.Context
	log    zerolog.Logger
	hubCom ws.HubCommunicator

	enableAuth  bool
	authCfg     map[string]config.AuthProvider
	serviceKeys map[string]noble.Secret
}

func (h handler) renderWebPage(w http.ResponseWriter, _ *http.Request) {
	rawPage, err := web.GetIndexPage()
	if err != nil {
		h.log.Error().Err(err).Msg("unable to read index page")
		render.ServerError(w)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(rawPage)
	if err != nil {
		h.log.Error().Err(err).Msg("unable to write index page")
		return
	}
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
	// logger := log.IncludeRequest(h.log, r)

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
		h.log.Error().Err(err).Msg("unable to upgrade http protocol")
		return
	}

	client := ws.NewSession(h.ctx, h.hubCom.LogProvider, h.hubCom.EventBus, conn, authInfo, h.authProvider)

	h.log.Debug().Msg("Open new client connection")
	h.hubCom.EventBus <- &ws.Event{Kind: ws.EKNewSession, Session: client}
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

	resp, err := httpx.NewXClient().PostJSON(provider.URL.Str, req,
		map[string]string{provider.Header: provider.AccessKey.Get()})
	if err != nil {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to check authorization")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errors.New("forbidden")
	}

	authInfo := new(models.AuthResponse)
	err = httpx.NewXClient().ParseJSONResult(resp, authInfo)
	if resp.StatusCode != http.StatusOK {
		return nil, http.StatusInternalServerError, errors.Wrap(err, "unable to parse auth response")
	}

	return authInfo, http.StatusOK, nil
}
