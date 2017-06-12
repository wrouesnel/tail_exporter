package main

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wrouesnel/tail_exporter/config"
)

// metricValue stores the typed value of a metric being collected by the
// exporter.
type metricValue struct {
	// desc is the prometheus description of this metric value.
	desc *prometheus.Desc
	// hash representing a structured interpretation of label values
	hash string
	// valueType is the prometheus TYPE of the generated metric
	valueType prometheus.ValueType
	// value is the current value of the internal metric
	value float64
	// metric timeout for GC purposes
	timeout time.Duration
	// stores the time of the last update for GC purposes
	lastUpdated time.Time
}

func newMetricValue(fqName string, help string, valueType config.MetricType, timeout time.Duration, labelPairs prometheus.Labels) (*metricValue, error) {
	metric := &metricValue{}

	switch valueType {
	case config.MetricUntyped:
		metric.valueType = prometheus.UntypedValue
	case config.MetricGauge:
		metric.valueType = prometheus.GaugeValue
	case config.MetricCounter:
		metric.valueType = prometheus.CounterValue
	default:
		return nil, fmt.Errorf("unknown metric value type: %s", valueType)
	}

	metric.desc = prometheus.NewDesc(fqName, help, []string{}, labelPairs)

	// Calculate the hash of the new metric from it's labels
	h := sha256.New()
	h.Write([]byte(metric.desc.String()))
	metric.hash = string(h.Sum(nil))

	metric.timeout = timeout

	return metric, nil
}

func (mv *metricValue) Describe(ch chan<- *prometheus.Desc) {
	ch <- mv.desc
}

func (mv *metricValue) Collect(ch chan<- prometheus.Metric) {
	// Metrics are dynamically generated when needed, because value updates
	// are common but scrapes are infrequent.
	// TODO: implement prometheus.Metric directly.
	ch <- prometheus.MustNewConstMetric(mv.desc, mv.valueType, mv.value)
}

// GetHash gets a cryptographically strong hash which describes the metric
// uniquely. This is currently SHA256 but is not considered a stable interface.
func (mv *metricValue) GetHash() string {
	return mv.hash
}

// Get returns the current value
func (mv *metricValue) Get() float64 {
	return mv.value
}

// Set sets the current value
func (mv *metricValue) Set(v float64) {
	// TODO: prevent counter from going < 0?
	mv.value = v
	mv.lastUpdated = time.Now()
}

// Sub decreases the stored value by v
func (mv *metricValue) Sub(v float64) {
	if mv.valueType == prometheus.CounterValue {
		mv.value = 0
	} else {
		mv.value -= v
	}
	mv.lastUpdated = time.Now()
}

// Add increases the stored value by v
func (mv *metricValue) Add(v float64) {
	mv.value += v
	// Check for an overflow
	if mv.value < 0 && mv.valueType == prometheus.CounterValue {
		mv.value = 0
	}
	mv.lastUpdated = time.Now()
}

// IsStale reports if the metric has exceeded its timeout, provided its timeout
// is greater then 0.
func (mv *metricValue) IsStale() bool {
	if mv.timeout > 0 {
		return time.Since(mv.lastUpdated) > mv.timeout
	}
	return false
}
