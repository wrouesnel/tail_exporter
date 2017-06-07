package main

import (
	"github.com/wrouesnel/tail_exporter/config"
	"github.com/glenn-brown/golang-pkg-pcre/src/pkg/pcre"
	"fmt"
	dto "github.com/prometheus/client_model/go"
	"github.com/golang/protobuf/proto"
	"strconv"
	"math"
)

func ParseLabelKey(def config.LabelValueDef, m *pcre.Matcher) (string, error) {
	switch def.FieldType {
	case config.LVALUE_LITERAL:
		return def.Literal, nil
	case config.LVALUE_CAPTUREGROUP_NAMED:
		if !m.NamedPresent(def.CaptureGroupName) {
			return "", fmt.Errorf("unconvertible capture value")
		}
		return m.NamedString(def.CaptureGroupName), nil
	case config.LVALUE_CAPTUREGROUP:
		return m.GroupString(def.CaptureGroup), nil
	default:
		return "", fmt.Errorf("unknown conversion type: %s", def.FieldType)
	}
}

// ParseLabelsFromMatch converts a regex match to a prometheus.Labels map. If
// a label can't be parsed at all it will be dropped, and the entire metric
// will be ignored for the given input match.
func ParseLabelPairsFromMatch(def []config.LabelDef, m *pcre.Matcher) ([]*dto.LabelPair, error) {
	// Initialize the LabelPair array
	lps := make([]*dto.LabelPair, len(def))
	for i:=0 ; i < len(def) ; i++ {
		lps[i] = &dto.LabelPair{}
	}

	// Calculate label names from the rule
	for idx, v := range def {
		name, nerr := ParseLabelKey(v.Name, m)
		if nerr != nil {
			return nil, fmt.Errorf("error parsing LabelDef for name")
		}

		value, verr := ParseLabelKey(v.Value, m)
		if verr != nil {
			return nil, fmt.Errorf("error parsing LabelDef for value")
		}

		lps[idx].Name = proto.String(name)
		lps[idx].Value = proto.String(value)
	}

	return lps, nil
}

// ParseValueFromMatch converts a regex match to a float64 suitable for use as
// a metric value, based on the value of a metric ValueDef. Returns NaN if a
// value is not convertible and an error.
func ParseValueFromMatch(def config.ValueDef, m *pcre.Matcher) (float64, error) {
	switch def.FieldType {
	case config.VALUE_LITERAL:
		return def.Literal, nil
	case config.VALUE_CAPTUREGROUP_NAMED:
		if !m.NamedPresent(def.CaptureGroupName) {
			return math.NaN(), fmt.Errorf("unconvertible capture value")
		}
		return m.NamedString(def.CaptureGroupName), nil
	case config.VALUE_CAPTUREGROUP:
		valstr := m.GroupString(def.CaptureGroup)
		val, err := strconv.ParseFloat(valstr, 64)
		return val, err
	case config.VALUE_INC:
		return 1, nil
	case config.VALUE_SUB:
		return -1, nil
	default:
		return math.NaN(), fmt.Errorf("unknown conversion type: %s", def.FieldType)
	}
}