// Defines the regexp file-type

package config

import (
	"fmt"
	"github.com/glenn-brown/golang-pkg-pcre/src/pkg/pcre"
	"gopkg.in/yaml.v2"
	"strings"
)

// flaggedRegex fields which fail to parse are parsed again against this to
// allow using the full PCRE library.
type flaggedRegex struct {
	regex string `yaml:"expr"`
	flags string `yaml:"flags,omitempty"`
}

// Regexp encapsulates a regexp.Regexp and makes it YAML marshallable.
type Regexp struct {
	pcre.Regexp
	original flaggedRegex
}

// RegexpCompileError wraps the pcre Compile error
type RegexpCompileError struct {
	*pcre.CompileError
}

func (this RegexpCompileError) Error() string {
	return this.CompileError.String()
}

// RegexpFlagsError provides a useful error message when bad flags are found
// for the supplied regexp
type RegexpFlagsError struct {
	badflag string
}

func (this RegexpFlagsError) Error() string {
	return fmt.Sprintf("Invalid regex flag specificed: %s", this.badflag)
}

func parseFlags(f string) (int, error) {
	// Early escape for usual case
	if f == "" {
		return 0, nil
	}

	flags := strings.Split(f, ",")
	var flag int
	for _, strflag := range flags {
		switch strflag {
		// Compile or match flags (we don't use match)
		case "anchored":
			flag |= pcre.ANCHORED
		case "bsr-anycrlf":
			flag |= pcre.BSR_ANYCRLF
		case "bsr-unicode":
			flag |= pcre.BSR_UNICODE
		case "newline-is-any":
			flag |= pcre.NEWLINE_ANY
		case "newline-is-anycrlf":
			flag |= pcre.NEWLINE_ANYCRLF
		case "newline-is-cr":
			flag |= pcre.NEWLINE_CR
		case "newline-crlf":
			flag |= pcre.NEWLINE_CRLF
		case "newline-lf":
			flag |= pcre.NEWLINE_LF
		case "no-utf8-check":
			flag |= pcre.NO_UTF8_CHECK
		// Compile-only flags
		case "caseless":
			flag |= pcre.CASELESS
		case "dollar-end-only":
			flag |= pcre.DOLLAR_ENDONLY
		case "dotall":
			flag |= pcre.DOTALL
		case "dupnames":
			flag |= pcre.DUPNAMES
		case "extended":
			flag |= pcre.EXTENDED
		case "extra":
			flag |= pcre.EXTRA
		case "firstline":
			flag |= pcre.FIRSTLINE
		case "javascript-compat":
			flag |= pcre.JAVASCRIPT_COMPAT
		case "multiline":
			flag |= pcre.MULTILINE
		case "no-auto-capture":
			flag |= pcre.NO_AUTO_CAPTURE
		case "ungreedy":
			flag |= pcre.UNGREEDY
		case "utf8":
			flag |= pcre.UTF8
		default:
			return 0, RegexpFlagsError{f}
		}
	}

	return flag, nil
}

// NewRegexp creates a new anchored Regexp and returns an error if the
// passed-in regular expression does not compile.
func NewRegexp(s flaggedRegex) (*Regexp, error) {
	flags, err := parseFlags(s.flags)
	if err != nil {
		return nil, err
	}

	regex, cerr := pcre.Compile(s.regex, flags)
	if cerr != nil {
		return nil, RegexpCompileError{cerr}
	}
	return &Regexp{
		Regexp:   regex,
		original: s,
	}, nil
}

// MustNewRegexp works like NewRegexp, but panics if the regular expression does not compile.
func MustNewRegexp(s flaggedRegex) *Regexp {
	re, err := NewRegexp(s)
	if err != nil {
		panic(err)
	}
	return re
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (re *Regexp) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var fr flaggedRegex
	var s string
	// Try parsing the full struct
	if err := unmarshal(&fr); err != nil {
		// Try parsing the short-form
		if err = unmarshal(&s); err != nil {
			// Fail
			return err
		}
		fr.regex = s
	}

	r, err := NewRegexp(fr)
	if err != nil {
		return err
	}
	*re = *r
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (re *Regexp) MarshalYAML() (interface{}, error) {
	if re.original.flags != "" {
		return yaml.Marshal(&re.original)
	} else if re != nil {
		return re.original, nil
	}
	return nil, nil
}
