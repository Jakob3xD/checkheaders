// Package checkheaders plugin.
package checkheaders

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// SingleHeader contains a single header keypair
type SingleHeader struct {
	Name      string   `json:"name,omitempty"`
	Values    []string `json:"values,omitempty"`
	MatchType string   `json:"matchtype,omitempty"`
	Required  *bool    `json:"required,omitempty"`
	Contains  *bool    `json:"contains,omitempty"`
	URLDecode *bool    `json:"urldecode,omitempty"`
	Debug     *bool    `json:"debug,omitempty"`
	Regex     *bool    `json:"regex,omitempty"` // New field for regex support
}

// Config the plugin configuration.
type Config struct {
	Headers []SingleHeader
}

// HeaderMatch demonstrates a HeaderMatch plugin.
type HeaderMatch struct {
	next    http.Handler
	headers []SingleHeader
	name    string
}

// MatchType defines an enum which can be used to specify the match type for the 'contains' config.
type MatchType string

const (
	//MatchAll requires all values to be matched
	MatchAll MatchType = "all"
	//MatchOne requires only one value to be matched
	MatchOne MatchType = "one"
	//MatchNone requires none of the values to be matched
	MatchNone MatchType = "none"
)

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Headers: []SingleHeader{},
	}
}

// New created a new HeaderMatch plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.Headers) == 0 {
		return nil, fmt.Errorf("configuration incorrect, missing headers")
	}

	for _, vHeader := range config.Headers {
		if strings.TrimSpace(vHeader.Name) == "" {
			return nil, fmt.Errorf("configuration incorrect, missing header name")
		}
		if len(vHeader.Values) == 0 {
			return nil, fmt.Errorf("configuration incorrect, missing header values")
		} else {
			for _, value := range vHeader.Values {
				if strings.TrimSpace(value) == "" {
					return nil, fmt.Errorf("configuration incorrect, empty value found")
				}
			}
		}
		if !vHeader.IsContains() && vHeader.MatchType == string(MatchAll) {
			return nil, fmt.Errorf("configuration incorrect for header %v %s", vHeader.Name, ", matchall can only be used in combination with 'contains'")
		}
		if strings.TrimSpace(vHeader.MatchType) == "" {
			return nil, fmt.Errorf("configuration incorrect, missing match type configuration for header %v", vHeader.Name)
		}
	}

	return &HeaderMatch{
		headers: config.Headers,
		next:    next,
		name:    name,
	}, nil
}

func (a *HeaderMatch) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	headersValid := true

	for _, vHeader := range a.headers {

		reqHeaderVal := req.Header.Get(vHeader.Name)

		if vHeader.IsURLDecode() {
			reqHeaderVal, _ = url.QueryUnescape(reqHeaderVal)
		}

		if reqHeaderVal != "" {
			if vHeader.IsContains() {
				headersValid = checkContains(&reqHeaderVal, &vHeader)
			} else if vHeader.IsRegex() {
				headersValid = checkRegex(&reqHeaderVal, &vHeader)
			} else {
				headersValid = checkRequired(&reqHeaderVal, &vHeader)
			}
		} else {
			headersValid = checkRequired(&reqHeaderVal, &vHeader)
		}

		if vHeader.IsDebug() {
			fmt.Println("checkheaders (debug): Headers valid:", headersValid)
			fmt.Println("checkheaders (debug): Request headers:", reqHeaderVal)
			fmt.Println("checkheaders (debug): Configured headers:", vHeader.Values)
		}

		if !headersValid {
			break
		}
	}

	if headersValid {
		a.next.ServeHTTP(rw, req)
	} else {
		http.Error(rw, "Not allowed", http.StatusForbidden)
	}
}

// checkContains checks whether a header value contains the configured value
func checkContains(requestValue *string, vHeader *SingleHeader) bool {

	if vHeader.IsDebug() {
		fmt.Println("checkheaders (debug): Validating contains:", *requestValue, vHeader.Values)
	}

	matchCount := 0
	for _, value := range vHeader.Values {
		if strings.Contains(*requestValue, value) {
			matchCount++
		}
	}

	if vHeader.MatchType == string(MatchNone) {
		return matchCount == 0
	}

	if matchCount == 0 {
		return false
	} else if vHeader.MatchType == string(MatchAll) && matchCount != len(vHeader.Values) {
		return false
	}

	return true
}

// checkRegex checks whether a header value matches the configured regex
func checkRegex(requestValue *string, vHeader *SingleHeader) bool {

	if vHeader.IsDebug() {
		fmt.Println("checkheaders (debug): Validating:", *requestValue, "with regex:", vHeader.Values)
	}

	matchCount := 0
	for _, value := range vHeader.Values {
		match, err := regexp.MatchString(value, *requestValue)

		if err == nil {
			if match {
				matchCount++
			}
		} else {
			if vHeader.IsDebug() {
				fmt.Println("checkheaders (debug): ERROR matching regex:", err)
			}
		}

	}

	if vHeader.MatchType == string(MatchNone) {
		return matchCount == 0
	}

	if matchCount == 0 {
		return false

	} else if vHeader.MatchType == string(MatchAll) && matchCount != len(vHeader.Values) {
		return false
	}

	return true
}

// checkRequired checks whether a header value is required in the request
// if the header is not required, it will also return true if the header is not present in the request
func checkRequired(requestValue *string, vHeader *SingleHeader) bool {

	if vHeader.IsDebug() {
		fmt.Println("checkheaders (debug): Validating required:", *requestValue, vHeader.Values)
	}

	matchCount := 0
	for _, value := range vHeader.Values {
		// if the header is required, it should match the configured value
		if *requestValue == value {
			matchCount++
		}

		if !vHeader.IsRequired() && *requestValue == "" {
			matchCount++
		}
	}

	if vHeader.MatchType == string(MatchNone) {
		return matchCount == 0
	}

	if matchCount == 0 {
		return false
	}

	return true
}

// IsURLDecode checks whether a header value should be url decoded first before testing it
func (s *SingleHeader) IsURLDecode() bool {
	if s.URLDecode == nil || !*s.URLDecode {
		return false
	}

	return true
}

// IsDebug checks whether a header value should print debug information in the log
func (s *SingleHeader) IsDebug() bool {
	if s.Debug == nil || !*s.Debug {
		return false
	}

	return true
}

// IsContains checks whether a header value should contain the configured value
func (s *SingleHeader) IsContains() bool {
	if s.Contains == nil || !*s.Contains {
		return false
	}

	return true
}

// IsRequired checks whether a header is mandatory in the request, defaults to 'true'
func (s *SingleHeader) IsRequired() bool {
	if s.Required == nil || *s.Required {
		return true
	}

	return false
}

// IsRegex checks whether a header value should be matched using regular expressions
func (s *SingleHeader) IsRegex() bool {
	if s.Regex == nil || !*s.Regex {
		return false
	}

	return true
}
