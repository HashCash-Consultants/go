package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shantanu-hashcash/go/exp/lightaurora/actions"
	"github.com/shantanu-hashcash/go/exp/lightaurora/services"
	supportHttp "github.com/shantanu-hashcash/go/support/http"
	"github.com/shantanu-hashcash/go/support/render/problem"
)

func newWrapResponseWriter(w http.ResponseWriter, r *http.Request) middleware.WrapResponseWriter {
	mw, ok := w.(middleware.WrapResponseWriter)
	if !ok {
		mw = middleware.NewWrapResponseWriter(w, r.ProtoMajor)
	}

	return mw
}

func prometheusMiddleware(requestDurationMetric *prometheus.SummaryVec) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := supportHttp.GetChiRoutePattern(r)
			mw := newWrapResponseWriter(w, r)

			then := time.Now()
			next.ServeHTTP(mw, r)
			duration := time.Since(then)

			requestDurationMetric.With(prometheus.Labels{
				"status": strconv.FormatInt(int64(mw.Status()), 10),
				"method": r.Method,
				"route":  route,
			}).Observe(float64(duration.Seconds()))
		})
	}
}

func lightAuroraHTTPHandler(registry *prometheus.Registry, lightAurora services.LightAurora) http.Handler {
	requestDurationMetric := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "aurora_lite", Subsystem: "http", Name: "requests_duration_seconds",
			Help: "HTTP requests durations, sliding window = 10m",
		},
		[]string{"status", "method", "route"},
	)
	registry.MustRegister(requestDurationMetric)

	router := chi.NewMux()
	router.Use(prometheusMiddleware(requestDurationMetric))

	router.Route("/accounts/{account_id}", func(r chi.Router) {
		r.MethodFunc(http.MethodGet, "/transactions", actions.NewTXByAccountHandler(lightAurora))
		r.MethodFunc(http.MethodGet, "/operations", actions.NewOpsByAccountHandler(lightAurora))
	})

	router.MethodFunc(http.MethodGet, "/", actions.Root(actions.RootResponse{
		Version: AuroraLiteVersion,
		// by default, no other fields are known yet
	}))
	router.MethodFunc(http.MethodGet, "/api", actions.ApiDocs())
	router.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	problem.RegisterHost("")
	router.NotFound(func(w http.ResponseWriter, request *http.Request) {
		problem.Render(request.Context(), w, problem.NotFound)
	})

	return router
}
