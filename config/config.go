package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/prometheus/common/model"
)

// Load parses the YAML input s into a Config.
func Load(s string) (*Config, error) {
	cfg := &Config{}

	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	cfg.Original = s
	return cfg, nil
}

// LoadFile parses the given YAML file into a Config.
func LoadFile(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

type Config struct {
	MetricConfigs []MetricParser `yaml:"metric_configs,omitempty"`

	// Catchall
	XXX map[string]string `yaml:",inline"`

	Original string
}

// Metric type definitions
type MetricType int

const (
	METRIC_UNTYPED MetricType = iota
	METRIC_GAUGE   MetricType = iota
	METRIC_COUNTER MetricType = iota
)

type ErrorInvalidMetricType struct{}

func (this ErrorInvalidMetricType) Error() string {
	return "Metric type must be 'gauge' or 'counter'"
}

func (this *MetricType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	switch s {
	case "gauge":
		*this = METRIC_GAUGE
	case "counter":
		*this = METRIC_COUNTER
	default:
		*this = METRIC_UNTYPED
	}
	return nil
}

func (this *MetricType) MarshalYAML() (interface{}, error) {
	switch *this {
	case METRIC_COUNTER:
		return "counter", nil
	case METRIC_GAUGE:
		return "gauge", nil
	default:
		return "invalid metric", nil
	}
}

type MetricParser struct {
	Name    string         `yaml:"name,omitempty"`
	Type    MetricType     `yaml:"type,omitempty"`
	Help    string         `yaml:"help,omitempty"`
	Regex   Regexp         `yaml:"regex,omitempty"`
	Labels  []LabelDef     `yaml:"labels,omitempty"`
	Value   ValueDef       `yaml:"value,omitempty"`
	Timeout model.Duration `yaml:"timeout,omitempty"`
}

type MetricParserErrorNoHelp struct{}

func (this MetricParserErrorNoHelp) Error() string {
	return "Metric help field cannot be empty."
}

func (this *MetricParser) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain MetricParser
	if err := unmarshal((*plain)(this)); err != nil {
		return err
	}

	if this.Help == "" {
		return &MetricParserErrorNoHelp{}
	}

	return nil
}

type LabelDef struct {
	Name  LabelValueDef `yaml:"name,omitempty"`
	Value LabelValueDef `yaml:"value,omitempty"`

	// Optional parameters get loaded into this map.
	optional map[string]string `yaml:",inline"`

	// Optional parameter: specify a default value for a missing key
	Default    string
	HasDefault bool
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (this *LabelDef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain LabelDef
	if err := unmarshal((*plain)(this)); err != nil {
		return err
	}

	// Populate optional values
	this.Default, this.HasDefault = this.optional["default"]
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (this *LabelDef) MarshalYAML() (interface{}, error) {
	// Set optional values into the map
	if this.HasDefault {
		this.optional["default"] = this.Default
	}
	type plain LabelDef
	return yaml.Marshal((*plain)(this))
}

type LabelValueType int

const (
	LVALUE_LITERAL            LabelValueType = iota
	LVALUE_CAPTUREGROUP       LabelValueType = iota
	LVALUE_CAPTUREGROUP_NAMED LabelValueType = iota
)

// Defines a type which sets ascii label values
type LabelValueDef struct {
	FieldType        LabelValueType
	Literal          string
	CaptureGroup     int
	CaptureGroupName string
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (this *LabelValueDef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	if strings.HasPrefix(s, "$") {
		// If we can match a number, assume a numbered group. If we can't, then
		// assume we are referring to a capture group name. If the name is invalid
		// then we'll fail to match but there's no easy way cross-validate with the
		// PCRE module just yet.
		str := strings.Trim(s, "$")
		val, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			this.FieldType = LVALUE_CAPTUREGROUP_NAMED
			this.CaptureGroupName = str
		} else {
			this.FieldType = LVALUE_CAPTUREGROUP
			this.CaptureGroup = int(val)
		}
	} else {
		this.FieldType = LVALUE_LITERAL
		this.Literal = s
	}
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (this *LabelValueDef) MarshalYAML() (interface{}, error) {
	switch this.FieldType {
	case LVALUE_CAPTUREGROUP:
		return fmt.Sprintf("$%d", this.CaptureGroup), nil
	default:
		return this.Literal, nil
	}
}

// Value definitions for label and value fields
type ValueType int

const (
	// Assign the specified value directly to the metric
	VALUE_LITERAL            ValueType = iota
	// Assign the value from the capture group to the metric
	VALUE_CAPTUREGROUP       ValueType = iota
	// Assign the value frm the given named capture group to the metric
	VALUE_CAPTUREGROUP_NAMED ValueType = iota
	// On every match increment the current metric value
	VALUE_INC                ValueType = iota
	// On every match decrement the current metric value.
	// (counters will simply reset to zero)
	VALUE_DEC                ValueType = iota
	// On every match increment the current metric by the received value
	VALUE_ADD				 ValueType = iota
	// On every match decrement the current metric by the received value
	// (counters will be reset to zero)
	VALUE_SUB				 ValueType = iota
)

// ValueDef is the definition for numeric values which will be assigned to metrics
type ValueDef struct {
	FieldType        ValueType
	Literal          float64
	CaptureGroup     int
	CaptureGroupName string
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (this *ValueDef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	if strings.HasPrefix(s, "$") {
		// If we can match a number, assume a numbered group. If we can't, then
		// assume we are refering to a capture group name. If the name is invalid
		// then we'll fail to match but there's no easy way cross-validate with the
		// PCRE module just yet.
		str := strings.Trim(s, "$")
		val, err := strconv.ParseInt(str, 10, 32)
		if err != nil {
			this.FieldType = VALUE_CAPTUREGROUP_NAMED
			this.CaptureGroupName = str
		} else {
			this.FieldType = VALUE_CAPTUREGROUP
			this.CaptureGroup = int(val)
		}

	} else if s == "increment" {
		this.FieldType = VALUE_INC
	} else if s == "decrement" {
		this.FieldType = VALUE_DEC
	} else if s == "add" {
		this.FieldType = VALUE_ADD
	} else if s == "subtract" {
		this.FieldType = VALUE_SUB
	} else {
		this.FieldType = VALUE_LITERAL

		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		this.Literal = val
	}
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface
func (this *ValueDef) MarshalYAML() (interface{}, error) {
	switch this.FieldType {
	case VALUE_CAPTUREGROUP:
		return fmt.Sprintf("$%d", this.CaptureGroup), nil
	case VALUE_INC:
		return "increment", nil
	case VALUE_SUB:
		return "decrement", nil
	case VALUE_LITERAL:
		return this.Literal, nil
	default:
		return this.Literal, nil
	}
}
