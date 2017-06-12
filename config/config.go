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
	MetricUntyped MetricType = iota
	MetricGauge   MetricType = iota
	MetricCounter MetricType = iota
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
		*this = MetricGauge
	case "counter":
		*this = MetricCounter
	default:
		*this = MetricUntyped
	}
	return nil
}

func (this *MetricType) MarshalYAML() (interface{}, error) {
	switch *this {
	case MetricCounter:
		return "counter", nil
	case MetricGauge:
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
	LabelValueLiteral           LabelValueType = iota
	LabelValueCaptureGroup      LabelValueType = iota
	LabelValueCaptureGroupNamed LabelValueType = iota
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
			this.FieldType = LabelValueCaptureGroupNamed
			this.CaptureGroupName = str
		} else {
			this.FieldType = LabelValueCaptureGroup
			this.CaptureGroup = int(val)
		}
	} else {
		this.FieldType = LabelValueLiteral
		this.Literal = s
	}
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (this *LabelValueDef) MarshalYAML() (interface{}, error) {
	switch this.FieldType {
	case LabelValueCaptureGroup:
		return fmt.Sprintf("$%d", this.CaptureGroup), nil
	default:
		return this.Literal, nil
	}
}

// ValueSourceType specifies the sourcex
type ValueOpType int

const (
	ValueOpAdd ValueOpType = iota
	ValueOpSubtract
	ValueOpEquals
)

type ValueSourceType int

const (
	// Assign the specified value directly to the metric
	ValueSourceLiteral ValueSourceType = iota
	// Assign the value from the capture group to the metric
	ValueSourceCaptureGroup
	// Assign the value frm the given named capture group to the metric
	ValueSourceNamedCaptureGroup
)

// ValueDef is the definition for numeric values which will be assigned to metrics
type ValueDef struct {
	ValueOp          ValueOpType
	ValueSource      ValueSourceType
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

	// Determine type of operation
	switch s[0] {
	case '+':
		this.ValueOp = ValueOpAdd
	case '-':
		this.ValueOp = ValueOpSubtract
	case '=':
		this.ValueOp = ValueOpEquals
	default:
		return fmt.Errorf("Value specification must start with one of +,-,=")
	}

	// Is this a capture group specification?
	if s[1] == '$' {
		// Capture group specification
		if val, err := strconv.ParseInt(string(s[2:]), 10, 64); err != nil {
			// Assume is named capture group
			this.ValueSource = ValueSourceNamedCaptureGroup
			this.CaptureGroupName = string(s[2:])
		} else {
			// Got a number - must be a numbered capture group
			this.ValueSource = ValueSourceCaptureGroup
			this.CaptureGroup = int(val)
		}
	} else {
		// Literal specification
		this.ValueSource = ValueSourceLiteral
		val, err := strconv.ParseFloat(string(s[1:]), 64)
		if err != nil {
			return fmt.Errorf("Could not parse literal float: %v", err)
		}
		this.Literal = val
	}

	return nil
}

// MarshalYAML implements the yaml.Marshaler interface
func (this *ValueDef) MarshalYAML() (interface{}, error) {
	var op, groupSpec, inputField string
	switch this.ValueOp {
	case ValueOpAdd:
		op = "+"
	case ValueOpSubtract:
		op = "-"
	case ValueOpEquals:
		op = "="
	default:
		return nil, fmt.Errorf("unknown value source specification in config")
	}

	switch this.ValueSource {
	case ValueSourceLiteral:
		groupSpec = ""
		inputField = fmt.Sprintf("%v", this.Literal)
	case ValueSourceCaptureGroup:
		groupSpec = "$"
		inputField = fmt.Sprintf("%d", this.CaptureGroup)
	case ValueSourceNamedCaptureGroup:
		groupSpec = "$"
		inputField = this.CaptureGroupName
	default:
		return nil, fmt.Errorf("unknown value source specification in config")
	}

	return fmt.Sprintf("%s%s%s", op, groupSpec, inputField), nil
}
