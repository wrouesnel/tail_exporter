package main

import (
	"fmt"
	"math"
	"strconv"

	"github.com/glenn-brown/golang-pkg-pcre/src/pkg/pcre"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/wrouesnel/tail_exporter/config"
)

func ParseLabelKey(def config.LabelValueDef, m *pcre.Matcher) (string, error) {
	switch def.FieldType {
	case config.LabelValueLiteral:
		return def.Literal, nil
	case config.LabelValueCaptureGroupNamed:
		if !m.NamedPresent(def.CaptureGroupName) {
			return "", fmt.Errorf("unconvertible capture value")
		}
		return m.NamedString(def.CaptureGroupName), nil
	case config.LabelValueCaptureGroup:
		return m.GroupString(def.CaptureGroup), nil
	default:
		return "", fmt.Errorf("unknown conversion type: %s", def.FieldType)
	}
}

// ParseLabelsFromMatch converts a regex match to a prometheus.Labels map. If
// a label can't be parsed at all it will be dropped, and the entire metric
// will be ignored for the given input match.
func ParseLabelPairsFromMatch(def []config.LabelDef, m *pcre.Matcher) (prometheus.Labels, error) {
	labels := make(prometheus.Labels, len(def))

	// Calculate label names from the rule
	for _, v := range def {
		name, nerr := ParseLabelKey(v.Name, m)
		if nerr != nil {
			return nil, fmt.Errorf("error parsing LabelDef for name")
		}

		value, verr := ParseLabelKey(v.Value, m)
		if verr != nil {
			return nil, fmt.Errorf("error parsing LabelDef for value")
		}

		labels[name] = value
	}

	return labels, nil
}

// ParseValueFromMatch converts a regex match to a float64 suitable for use as
// a metric value, based on the value of a metric ValueDef. Returns NaN if a
// value is not convertible and an error.
func ParseValueFromMatch(def config.ValueDef, m *pcre.Matcher) (float64, error) {
	switch def.ValueSource {
	case config.ValueSourceLiteral:
		return def.Literal, nil
	case config.ValueSourceNamedCaptureGroup:
		if !m.NamedPresent(def.CaptureGroupName) {
			return math.NaN(), fmt.Errorf("named capture group not present")
		}
		valstr := m.NamedString(def.CaptureGroupName)
		val, err := strconv.ParseFloat(valstr, 64)
		return val, err
	case config.ValueSourceCaptureGroup:
		if !m.Present(def.CaptureGroup) {
			return math.NaN(), fmt.Errorf("capture group not present")
		}
		valstr := m.GroupString(def.CaptureGroup)
		val, err := strconv.ParseFloat(valstr, 64)
		return val, err
	default:
		return math.NaN(), fmt.Errorf("unknown conversion type: %s", def.ValueSource)
	}
}
