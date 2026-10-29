package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	registry         *prometheus.Registry
	requests         *prometheus.CounterVec
	duration         *prometheus.HistogramVec
	orders           *prometheus.CounterVec
	revenue          prometheus.Counter
	cartAdds         prometheus.Counter
	checkoutFailures prometheus.Counter
	healthFailures   *prometheus.CounterVec
}

func newMetrics(reg *prometheus.Registry, version string) *metrics {
	m := &metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Requisições HTTP por rota e status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Tempo de resposta das requisições HTTP.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		orders: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "lojagamer_orders_total",
			Help: "Pedidos finalizados por forma de pagamento.",
		}, []string{"payment_method"}),
		revenue: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lojagamer_revenue_reais_total",
			Help: "Soma do valor dos pedidos finalizados, em reais.",
		}),
		cartAdds: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lojagamer_cart_additions_total",
			Help: "Unidades adicionadas ao carrinho.",
		}),
		checkoutFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "lojagamer_checkout_failures_total",
			Help: "Tentativas de checkout recusadas (carrinho vazio, sem estoque, erro).",
		}),
		healthFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "lojagamer_health_check_failures_total",
			Help: "Falhas de health check por dependência.",
		}, []string{"check"}),
	}
	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "lojagamer_build_info",
		Help:        "Versão (commit) em execução.",
		ConstLabels: prometheus.Labels{"version": version},
	})
	buildInfo.Set(1)

	reg.MustRegister(m.requests, m.duration, m.orders, m.revenue, m.cartAdds, m.checkoutFailures,
		m.healthFailures, buildInfo,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *metrics) observe(method, route string, status int, d time.Duration) {
	m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(method, route).Observe(d.Seconds())
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
