package http

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// FieldError describes a validation failure for a single struct field.
type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// Validator is an optional interface that request structs can implement
// for custom validation logic (e.g., cross-field checks). Called after
// struct-tag validation passes. Return an *APIError or any error to
// signal failure.
type Validator interface {
	Validate() error
}

// Validate checks dst for validation errors using `validate` struct tags.
// Returns nil if all fields pass, or an *APIError (status 422) with
// field-level details on failure. dst must be a pointer to a struct.
//
// If dst implements the Validator interface, its Validate method is
// called after tag-based validation passes.
//
// Supported rules:
//
//	required      field must be non-zero
//	min=N         strings: min rune length; numbers: min value
//	max=N         strings: max rune length; numbers: max value
//	oneof=a b c   value must be one of the space-separated options
//	email         basic email format (local@domain.tld)
//	url           parseable as an absolute URL with scheme and host
//
// When a field is zero-value and not marked required, all other rules
// are skipped for that field. Pointer fields: nil = zero (required
// check); non-nil = dereference and validate the pointed-to value.
// Embedded structs are recursed into.
//
//	type CreateReq struct {
//	    Email string `json:"email" validate:"required,email"`
//	    Name  string `json:"name"  validate:"required,min=2,max=100"`
//	    Role  string `json:"role"  validate:"oneof=admin user"`
//	}
func Validate(dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	specs := cachedSpecs(rv.Type())

	if len(specs) == 0 {
		if v, ok := dst.(Validator); ok {
			return v.Validate()
		}
		return nil
	}

	var errs []FieldError
	for _, fs := range specs {
		fv := rv.FieldByIndex(fs.index)
		errs = checkField(fv, fs, errs)
	}

	if len(errs) > 0 {
		return ErrValidation.WithDetails(errs)
	}

	if v, ok := dst.(Validator); ok {
		return v.Validate()
	}

	return nil
}

// --- Rule execution ---

func checkField(fv reflect.Value, fs fieldSpec, errs []FieldError) []FieldError {
	isZero := fv.IsZero()

	if isZero {
		if fs.required {
			errs = append(errs, FieldError{
				Field:   fs.name,
				Rule:    "required",
				Message: fs.name + " is required",
			})
		}
		return errs
	}

	// Dereference pointer for rule checks.
	v := fv
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	for _, r := range fs.rules {
		if msg := runRule(v, fs.name, r); msg != "" {
			errs = append(errs, FieldError{
				Field:   fs.name,
				Rule:    r.name,
				Message: msg,
			})
		}
	}
	return errs
}

func runRule(fv reflect.Value, name string, r rule) string {
	switch r.name {
	case "min":
		return checkMin(fv, name, r.param)
	case "max":
		return checkMax(fv, name, r.param)
	case "oneof":
		return checkOneof(fv, name, r.param)
	case "email":
		return checkEmail(fv, name)
	case "url":
		return checkURL(fv, name)
	}
	return ""
}

func checkMin(fv reflect.Value, name, param string) string {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return ""
	}
	switch fv.Kind() {
	case reflect.String:
		if len([]rune(fv.String())) < int(n) {
			return fmt.Sprintf("%s must be at least %s characters", name, param)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(fv.Int()) < n {
			return fmt.Sprintf("%s must be at least %s", name, param)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(fv.Uint()) < n {
			return fmt.Sprintf("%s must be at least %s", name, param)
		}
	case reflect.Float32, reflect.Float64:
		if fv.Float() < n {
			return fmt.Sprintf("%s must be at least %s", name, param)
		}
	}
	return ""
}

func checkMax(fv reflect.Value, name, param string) string {
	n, err := strconv.ParseFloat(param, 64)
	if err != nil {
		return ""
	}
	switch fv.Kind() {
	case reflect.String:
		if len([]rune(fv.String())) > int(n) {
			return fmt.Sprintf("%s must be at most %s characters", name, param)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if float64(fv.Int()) > n {
			return fmt.Sprintf("%s must be at most %s", name, param)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if float64(fv.Uint()) > n {
			return fmt.Sprintf("%s must be at most %s", name, param)
		}
	case reflect.Float32, reflect.Float64:
		if fv.Float() > n {
			return fmt.Sprintf("%s must be at most %s", name, param)
		}
	}
	return ""
}

func checkOneof(fv reflect.Value, name, param string) string {
	s := fmt.Sprint(fv.Interface())
	for _, opt := range strings.Split(param, " ") {
		if s == opt {
			return ""
		}
	}
	return fmt.Sprintf("%s must be one of: %s", name, strings.ReplaceAll(param, " ", ", "))
}

func checkEmail(fv reflect.Value, name string) string {
	if fv.Kind() != reflect.String {
		return ""
	}
	s := fv.String()
	at := strings.IndexByte(s, '@')
	if at < 1 || at >= len(s)-1 {
		return name + " must be a valid email address"
	}
	domain := s[at+1:]
	dot := strings.IndexByte(domain, '.')
	if dot < 1 || dot >= len(domain)-1 {
		return name + " must be a valid email address"
	}
	return ""
}

func checkURL(fv reflect.Value, name string) string {
	if fv.Kind() != reflect.String {
		return ""
	}
	u, err := url.ParseRequestURI(fv.String())
	if err != nil || u.Scheme == "" || u.Host == "" {
		return name + " must be a valid URL"
	}
	return ""
}

// --- Tag parsing & caching ---

type fieldSpec struct {
	index    []int // field path for FieldByIndex (handles embedded structs)
	name     string
	required bool
	rules    []rule
}

type rule struct {
	name  string // "min", "max", "oneof", "email", "url"
	param string // e.g. "3" for min=3, "admin user" for oneof=admin user
}

var specCache sync.Map // reflect.Type → []fieldSpec

func cachedSpecs(t reflect.Type) []fieldSpec {
	if v, ok := specCache.Load(t); ok {
		return v.([]fieldSpec)
	}
	specs := parseSpecs(t, nil)
	v, _ := specCache.LoadOrStore(t, specs)
	return v.([]fieldSpec)
}

func parseSpecs(t reflect.Type, parent []int) []fieldSpec {
	var specs []fieldSpec
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		idx := make([]int, len(parent)+1)
		copy(idx, parent)
		idx[len(parent)] = i

		// Recurse into embedded structs.
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			specs = append(specs, parseSpecs(field.Type, idx)...)
			continue
		}

		tag := field.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}

		required, rules := parseRules(tag)
		if !required && len(rules) == 0 {
			continue
		}

		specs = append(specs, fieldSpec{
			index:    idx,
			name:     resolveFieldName(field),
			required: required,
			rules:    rules,
		})
	}
	return specs
}

func parseRules(tag string) (required bool, rules []rule) {
	for _, p := range strings.Split(tag, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "required" {
			required = true
			continue
		}
		name, param, _ := strings.Cut(p, "=")
		rules = append(rules, rule{name: name, param: param})
	}
	return
}

func resolveFieldName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if name, _, _ := strings.Cut(tag, ","); name != "" {
			return name
		}
	}
	if tag := f.Tag.Get("query"); tag != "" && tag != "-" {
		return tag
	}
	if len(f.Name) > 0 {
		return strings.ToLower(f.Name[:1]) + f.Name[1:]
	}
	return f.Name
}
