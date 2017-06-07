package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"fmt"
	"crypto/sha256"
	"github.com/wrouesnel/tail_exporter/config"
	"sort"
	dto "github.com/prometheus/client_model/go"
	"time"
)

// metricValue stores the typed value of a metric being collected by the
// exporter.
type metricValue struct {
	fqName		string
	help        string
	labelPairs  []*dto.LabelPair // Sorted label-pairs
	valueType   prometheus.ValueType

	hash 		string // SHA256 hash representing a structured interpretation of label values

	value float64
	lastUpdated time.Time
}

func newMetricValue(fqName string, help string, valueType config.MetricType, labelPairs... *dto.LabelPair) (*metricValue, error) {
	metric := metricValue{
		fqName:     fqName,
		help:       help,
		labelPairs: labelPairs,
	}
	sort.Sort(prometheus.LabelPairSorter{metric.labelPairs})
	switch valueType {
	case config.METRIC_UNTYPED:
		metric.valueType = prometheus.UntypedValue
	case config.METRIC_GAUGE:
		metric.valueType = prometheus.GaugeValue
	case config.METRIC_COUNTER:
		metric.valueType = prometheus.CounterValue
	default:
		return nil, fmt.Errorf("unknown metric value type: %s", valueType)
	}

	// Calculate the hash of the new metric from it's labels
	h := sha256.New()
	h.Write([]byte(fqName))
	// These are sorted above, so the hash will always match if its known
	// elsewhere
	for _, labelPair := range metric.labelPairs {
		h.Write([]byte(labelPair.GetName()))
		h.Write([]byte(labelPair.GetValue()))
	}
	metric.hash = string(h.Sum(nil))

	return &metric, nil
}

// GetHash gets a cryptographically strong hash which describes the metric
// uniquely. This is currently SHA256 but is not considered a stable interface.
//func (mv *metricValue) GetHash() []byte {
//	uniqueFmt := fmt.Sprintf("%s %s %s",
//		mv.fqName, mv.help, strings.Join(mv.labelNames, ","))
//
//	h := sha256.New()
//	return h.Sum([]byte(uniqueFmt))
//}