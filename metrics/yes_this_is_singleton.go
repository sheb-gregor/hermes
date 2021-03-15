package metrics

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/lancer-kit/uwe/v2"
	"github.com/lancer-kit/uwe/v2/presets/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//nolint:gochecknoglobals
var (
	enabled   bool
	collector Collector
)

type CollectorOpts struct{}

func Init(_ CollectorOpts) {
	enabled = true
	collector = Collector{
		gauges: map[MKey]prometheus.Gauge{},
	}
}

// Inc increments the Gauge by 1. Use Add to increment it by arbitrary
// values.
func Inc(name MKey) {
	if !enabled {
		return
	}
	collector.gauges[name].Inc()
}

// Dec decrements the Gauge by 1. Use Sub to decrement it by arbitrary
// values.
func Dec(name MKey) {
	if !enabled {
		return
	}
	collector.gauges[name].Dec()
}

// Add adds the given value to the Gauge. (The value can be negative,
// resulting in a decrease of the Gauge.)
func Add(name MKey, val float64) {
	if !enabled {
		return
	}
	collector.gauges[name].Add(val)
}

// Sub subtracts the given value from the Gauge. (The value can be
// negative, resulting in an increase of the Gauge.)
func Sub(name MKey, val float64) {
	if !enabled {
		return
	}
	collector.gauges[name].Sub(val)
}

// Set sets the Gauge to an arbitrary value.
func Set(name MKey, val float64) {
	if !enabled {
		return
	}
	collector.gauges[name].Add(val)
}

type Collector struct {
	Service string
	Host    string
	gauges  map[MKey]prometheus.Gauge
}

func RegisterGauge(name MKey) {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        string(name),
			ConstLabels: map[string]string{},
		})
	prometheus.MustRegister(gauge)
	collector.gauges[name] = gauge
}

func RegisterGauges(names ...MKey) {
	for _, name := range names {
		gauge := prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        string(name),
				ConstLabels: map[string]string{},
			})
		prometheus.MustRegister(gauge)
		collector.gauges[name] = gauge
	}
}

func GetMonitoringServer(cfg MonitoringConf, app uwe.AppInfo) *api.Server {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	if cfg.Metrics {
		r.Handle("/metrics", promhttp.Handler())
	}

	if cfg.PPROF {
		r.Mount("/debug", middleware.Profiler())
	}

	r.Get("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(app)
	})

	return api.NewServer(cfg.API, r)
}

func GetMonitoringMux(cfg MonitoringConf) http.Handler {
	r := chi.NewRouter()

	if cfg.Metrics {
		r.Handle("/metrics", promhttp.Handler())
	}

	if cfg.PPROF {
		r.Mount("/debug", middleware.Profiler())
	}

	// r.Get("/metrics", func(writer http.ResponseWriter, _ *http.Request) {
	// 	data, _ := socket.MetricsCollector.MarshalJSON()
	// 	writer.WriteHeader(200)
	// 	_, _ = writer.Write(data)
	// })

	return r
}

type MonitoringConf struct {
	Metrics bool       `json:"metrics" yaml:"metrics"`
	PPROF   bool       `json:"pprof" yaml:"pprof"`
	API     api.Config `json:"api" yaml:"api"`
	Service string     `json:"service" yaml:"service"`
	Host    string     `json:"host" yaml:"host"`
}

func (cfg MonitoringConf) Validate() error {
	if !cfg.Metrics && !cfg.PPROF {
		return nil
	}
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.API, validation.Required),
		validation.Field(&cfg.Service, validation.Required),
		validation.Field(&cfg.Host, validation.Required),
	)
}
