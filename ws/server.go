package ws

import (
	"context"
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
	"gitlab.inn4science.com/ctp/hermes/info"
	"gitlab.inn4science.com/ctp/hermes/ws/socket"
)

func GetServer(logger *logrus.Entry, cfg api.Config, ctx context.Context, bus socket.EventStream) *api.Server {
	return api.NewServer(cfg, getRouter(ctx, logger, cfg, bus))
}

func getRouter(ctx context.Context, logger *logrus.Entry, config api.Config, bus socket.EventStream) http.Handler {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(log.NewRequestLogger(logger.Logger))

	if config.EnableCORS {
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

	r.Route("/_ws", func(r chi.Router) {
		r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
			render.Success(w, info.App)
		})

		r.Get("/subscribe", handleNewWS(ctx, logger, bus))
		r.Get("/gc", func(http.ResponseWriter, *http.Request) { runtime.GC() })

		// r.Get("/metrics", func(writer http.ResponseWriter, _ *http.Request) {
		// 	data, _ := socket.MetricsCollector.MarshalJSON()
		// 	writer.WriteHeader(200)
		// 	_, _ = writer.Write(data)
		// })

	})
	return r
}

func handleNewWS(ctx context.Context, log *logrus.Entry, bus socket.EventStream) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Socket open with method:", r.Method)

		sessionUID := time.Now().UnixNano()
		// MetricsCollector.Add("ws.client")
		userUID := r.URL.Query().Get("uuid")
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
			log.WithError(err).Error("unable to upgrade http protocol")
			return
		}

		client := socket.NewSession(ctx, log, bus, conn, sessionUID, userUID)
		log.WithField("uuid", userUID).Debug("Open new client connection")
		bus <- &socket.Event{Kind: socket.EKNewSession, Session: client}
	}
}
