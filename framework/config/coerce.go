package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// coerce converts a raw value to the target type T.
// Returns (value, true) on success, or (zero, false) on failure.
func coerce[T any](raw any) (T, bool) {
	if v, ok := raw.(T); ok {
		return v, true
	}

	var zero T
	switch any(zero).(type) {
	case string:
		v := toString(raw)
		return any(v).(T), true
	case bool:
		v, ok := toBool(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case int:
		v, ok := toInt(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case int32:
		v, ok := toInt32(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case int64:
		v, ok := toInt64(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case uint:
		v, ok := toUint(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case uint8:
		v, ok := toUint8(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case uint16:
		v, ok := toUint16(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case uint32:
		v, ok := toUint32(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case uint64:
		v, ok := toUint64(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case float64:
		v, ok := toFloat64(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case time.Time:
		v, ok := toTime(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case time.Duration:
		v, ok := toDuration(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case []int:
		v, ok := toIntSlice(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case []string:
		v, ok := toStringSlice(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case map[string]any:
		v, ok := toStringMap(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case map[string]string:
		v, ok := toStringMapString(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	case map[string][]string:
		v, ok := toStringMapStringSlice(raw)
		if !ok {
			return zero, false
		}
		return any(v).(T), true
	default:
		return zero, false
	}
}

// ---------------------------------------------------------------------------
// Scalar coercion
// ---------------------------------------------------------------------------

func toString(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(v) {
		case "true", "1", "t", "yes", "on":
			return true, true
		case "false", "0", "f", "no", "off":
			return false, true
		default:
			return false, false
		}
	case int:
		return v != 0, true
	case float64:
		return v != 0, true
	default:
		return false, false
	}
}

func toInt(raw any) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		if v > math.MaxInt || v < math.MinInt {
			return 0, false
		}
		return int(v), true
	case float64:
		if v > float64(math.MaxInt) || v < float64(math.MinInt) {
			return 0, false
		}
		return int(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		if n > int64(math.MaxInt) || n < int64(math.MinInt) {
			return 0, false
		}
		return int(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toInt32(raw any) (int32, bool) {
	switch v := raw.(type) {
	case int32:
		return v, true
	case int:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, false
		}
		return int32(v), true
	case int64:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, false
		}
		return int32(v), true
	case float64:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, false
		}
		return int32(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return 0, false
		}
		return int32(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toInt64(raw any) (int64, bool) {
	switch v := raw.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		if v > float64(math.MaxInt64) || v < float64(math.MinInt64) {
			return 0, false
		}
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toUint(raw any) (uint, bool) {
	switch v := raw.(type) {
	case uint:
		return v, true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint(v), true
	case float64:
		if v < 0 || v > float64(math.MaxUint) {
			return 0, false
		}
		return uint(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return uint(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toUint8(raw any) (uint8, bool) {
	switch v := raw.(type) {
	case uint8:
		return v, true
	case int:
		if v < 0 || v > math.MaxUint8 {
			return 0, false
		}
		return uint8(v), true
	case int64:
		if v < 0 || v > math.MaxUint8 {
			return 0, false
		}
		return uint8(v), true
	case float64:
		if v < 0 || v > math.MaxUint8 {
			return 0, false
		}
		return uint8(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return 0, false
		}
		return uint8(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toUint16(raw any) (uint16, bool) {
	switch v := raw.(type) {
	case uint16:
		return v, true
	case int:
		if v < 0 || v > math.MaxUint16 {
			return 0, false
		}
		return uint16(v), true
	case int64:
		if v < 0 || v > math.MaxUint16 {
			return 0, false
		}
		return uint16(v), true
	case float64:
		if v < 0 || v > math.MaxUint16 {
			return 0, false
		}
		return uint16(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return 0, false
		}
		return uint16(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toUint32(raw any) (uint32, bool) {
	switch v := raw.(type) {
	case uint32:
		return v, true
	case int:
		if v < 0 {
			return 0, false
		}
		if uint64(v) > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		if uint64(v) > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case float64:
		if v < 0 || v > math.MaxUint32 {
			return 0, false
		}
		return uint32(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, false
		}
		return uint32(n), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toUint64(raw any) (uint64, bool) {
	switch v := raw.(type) {
	case uint64:
		return v, true
	case uint:
		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toFloat64(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func toTime(raw any) (time.Time, bool) {
	switch v := raw.(type) {
	case time.Time:
		return v, true
	case string:
		for _, layout := range []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02",
			"2006-01-02 15:04:05",
		} {
			t, err := time.Parse(layout, v)
			if err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}

func toDuration(raw any) (time.Duration, bool) {
	switch v := raw.(type) {
	case time.Duration:
		return v, true
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			// Try parsing as a bare number (nanoseconds).
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, false
			}
			return time.Duration(n), true
		}
		return d, true
	case int:
		return time.Duration(v), true
	case int64:
		return time.Duration(v), true
	case float64:
		return time.Duration(int64(v)), true
	default:
		return 0, false
	}
}

// ---------------------------------------------------------------------------
// Collection coercion
// ---------------------------------------------------------------------------

func toIntSlice(raw any) ([]int, bool) {
	switch v := raw.(type) {
	case []int:
		return v, true
	case []any:
		result := make([]int, 0, len(v))
		for _, item := range v {
			n, ok := toInt(item)
			if !ok {
				return nil, false
			}
			result = append(result, n)
		}
		return result, true
	case string:
		if v == "" {
			return nil, false
		}
		parts := strings.Split(v, ",")
		result := make([]int, 0, len(parts))
		for _, p := range parts {
			n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				return nil, false
			}
			result = append(result, int(n))
		}
		return result, true
	default:
		return nil, false
	}
}

func toStringSlice(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result, true
	case string:
		if v == "" {
			return nil, false
		}
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			result = append(result, strings.TrimSpace(p))
		}
		return result, true
	default:
		return nil, false
	}
}

func toStringMap(raw any) (map[string]any, bool) {
	if v, ok := raw.(map[string]any); ok {
		return v, true
	}
	return nil, false
}

func toStringMapString(raw any) (map[string]string, bool) {
	switch v := raw.(type) {
	case map[string]string:
		return v, true
	case map[string]any:
		result := make(map[string]string, len(v))
		for k, val := range v {
			result[k] = fmt.Sprintf("%v", val)
		}
		return result, true
	default:
		return nil, false
	}
}

func toStringMapStringSlice(raw any) (map[string][]string, bool) {
	switch v := raw.(type) {
	case map[string][]string:
		return v, true
	case map[string]any:
		result := make(map[string][]string, len(v))
		for k, val := range v {
			switch s := val.(type) {
			case []any:
				strs := make([]string, 0, len(s))
				for _, item := range s {
					strs = append(strs, fmt.Sprintf("%v", item))
				}
				result[k] = strs
			case []string:
				result[k] = s
			default:
				result[k] = []string{fmt.Sprintf("%v", val)}
			}
		}
		return result, true
	default:
		return nil, false
	}
}
