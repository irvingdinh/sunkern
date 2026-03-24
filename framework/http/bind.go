package http

import (
	"encoding/json"
	"fmt"
	httpstd "net/http"
	"reflect"
	"strconv"
)

// Bind decodes the JSON request body into dst. Returns an *APIError with
// status 400 if the body is missing or malformed. dst must be a pointer.
func Bind(r *httpstd.Request, dst any) error {
	if r.Body == nil {
		return ErrBadRequest.WithMessage("Request body is required")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return ErrBadRequest.WithMessage("Invalid request body: " + err.Error())
	}
	return nil
}

// BindQuery parses URL query parameters into dst using `query` struct tags.
// Supported field types: string, int, int64, uint, uint64, float64, bool.
// Fields without a matching query parameter retain their current value —
// set struct field defaults before calling BindQuery.
//
//	q := ListParams{Page: 1, PerPage: 20}
//	if err := sunkernhttp.BindQuery(r, &q); err != nil { ... }
func BindQuery(r *httpstd.Request, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return ErrInternal.WithMessage("BindQuery: dst must be a pointer to a struct")
	}
	rv = rv.Elem()
	rt := rv.Type()

	params := r.URL.Query()
	for i := range rt.NumField() {
		field := rt.Field(i)
		tag := field.Tag.Get("query")
		if tag == "" || tag == "-" {
			continue
		}
		val := params.Get(tag)
		if val == "" {
			continue
		}
		if err := setFieldFromString(rv.Field(i), val); err != nil {
			return ErrBadRequest.WithMessage(fmt.Sprintf("Invalid query parameter %q: %s", tag, err.Error()))
		}
	}
	return nil
}

func setFieldFromString(fv reflect.Value, s string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected integer")
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected positive integer")
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected number")
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("expected boolean")
		}
		fv.SetBool(b)
	default:
		return fmt.Errorf("unsupported type %s", fv.Type())
	}
	return nil
}
