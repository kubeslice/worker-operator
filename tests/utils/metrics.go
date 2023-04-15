package utils

import (
	"github.com/prometheus/client_golang/prometheus"
)

func ResetMetricRegistry(r *prometheus.Registry) {
	mf, _ := r.Gather()
	for _, m := range mf {
		m.Reset()
	}
}

func GetCounterMetricFromRegistry(r *prometheus.Registry, name string) []float64 {
	mf, _ := r.Gather()
	values := []float64{}
	for _, m := range mf {
		if *m.Name == name {
			for _, v := range m.Metric {
				values = append(values, v.Counter.GetValue())
			}
		}
	}
	return values
}

func GetGaugeMetricFromRegistry(r *prometheus.Registry, name string) []float64 {
	mf, _ := r.Gather()
	values := []float64{}
	for _, m := range mf {
		if *m.Name == name {
			for _, v := range m.Metric {
				values = append(values, v.Gauge.GetValue())
			}
		}
	}
	return values
}
