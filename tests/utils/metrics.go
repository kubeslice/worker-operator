package utils

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func GetCounterMetricFromRegistry(r *prometheus.Registry, name string, labels map[string]string) (float64, error) {
	mf, _ := r.Gather()
	var av []*dto.Metric
	for _, m := range mf {
		if *m.Name == name {
			for _, v := range m.Metric {
				av = m.GetMetric()
				if matchLabels(v.Label, labels) {
					return v.Counter.GetValue(), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no matching metric; available metrics: %v", av)
}

func GetGaugeMetricFromRegistry(r *prometheus.Registry, name string, labels map[string]string) (float64, error) {
	mf, _ := r.Gather()
	var av []*dto.Metric
	for _, m := range mf {
		if *m.Name == name {
			for _, v := range m.Metric {
				av = m.GetMetric()
				if matchLabels(v.Label, labels) {
					return v.Gauge.GetValue(), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no matching metric; available metrics: %v", av)
}

func matchLabels(lp []*dto.LabelPair, ml map[string]string) bool {

	for k, v := range ml {
		found := false

		for _, l := range lp {
			if k == *l.Name && v == l.GetValue() {
				found = true
				break
			}
		}

		if !found {
			return false
		}

	}

	return true
}
