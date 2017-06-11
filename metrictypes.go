package main

import (
	"crypto/sha256"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/wrouesnel/tail_exporter/config"
	"time"
)

// metricValue stores the typed value of a metric being collected by the
// exporter.
type metricValue struct {
	fqName     string
	help       string
	labelPairs []*dto.LabelPair // Sorted label-pairs
	valueType  prometheus.ValueType

	hash string // SHA256 hash representing a structured interpretation of label values

	// Desc is the prometheus description of this metric value.
	desc *prometheus.Desc

	value       float64
	lastUpdated time.Time
}

func newMetricValue(fqName string, help string, valueType config.MetricType, labelPairs prometheus.Labels) (*metricValue, error) {
	metric := &metricValue{}

	//sort.Sort(prometheus.LabelPairSorter{metric.labelPairs})
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

	metric.desc = prometheus.NewDesc(fqName, help, nil, labelPairs)

	// Calculate the hash of the new metric from it's labels
	h := sha256.New()
	h.Write([]byte(metric.desc.String()))
	metric.hash = string(h.Sum(nil))
	return metric, nil
}

func (mv *metricValue) Describe(chan<- *prometheus.Desc) {

}

func (mv *metricValue) Collect(chan<- prometheus.Metric) {

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
}

// Sub decreases the stored value by v
func (mv *metricValue) Sub(v float64) {
	if mv.valueType == prometheus.CounterValue {
		mv.value = 0
	} else {
		mv.value -= v
	}
}

// Add increases the stored value by v
func (mv *metricValue) Add(v float64) {
	mv.value += v
	// Check for an overflow
	if mv.value < 0 && mv.valueType == prometheus.CounterValue {
		mv.value = 0
	}
}
