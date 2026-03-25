---
title: Avoid JSON Encoding Surprises
impact: MEDIUM-HIGH
impactDescription: prevents encoding surprises
tags: json, encoding, time, embedding
---

## Avoid JSON Encoding Surprises

**Impact: MEDIUM-HIGH (prevents encoding surprises)**

Three common JSON gotchas in Go:

**A) Embedded types can override marshaling.** When you embed a type that implements `json.Marshaler` (like `time.Time`), its `MarshalJSON` method takes over the entire struct's marshaling, silently dropping other fields.

**B) `time.Time` contains a monotonic clock reading.** Two `time.Time` values representing the same instant may not be `==` equal because of different monotonic readings. Always use `time.Equal()` for comparison.

**C) Unmarshaling into `map[string]any` converts all numbers to `float64`.** JSON has no integer type, so the decoder defaults to `float64`, which can lose precision for large integers.

**Incorrect (what's wrong):**

```go
// A) Embedded time.Time hijacks marshaling
type Event struct {
	time.Time
	ID int
}

e := Event{Time: time.Now(), ID: 1}
b, _ := json.Marshal(e) // Only marshals the time, ID is lost

// B) Comparing time with ==
t1 := time.Now()
t2, _ := time.Parse(time.RFC3339, t1.Format(time.RFC3339))
if t1 == t2 { // May be false even for the same instant
	fmt.Println("equal")
}

// C) Numbers become float64
var result map[string]any
json.Unmarshal([]byte(`{"id": 1234567890123456789}`), &result)
id := result["id"].(float64) // Precision lost for large integers
```

**Correct (what's right):**

```go
// A) Use named fields instead of embedding
type Event struct {
	Time time.Time `json:"time"`
	ID   int       `json:"id"`
}

e := Event{Time: time.Now(), ID: 1}
b, _ := json.Marshal(e) // Both fields are marshaled correctly

// B) Use time.Equal() for comparison
t1 := time.Now()
t2, _ := time.Parse(time.RFC3339, t1.Format(time.RFC3339))
if t1.Equal(t2) { // Correctly compares the time instant
	fmt.Println("equal")
}

// C) Use json.Decoder with UseNumber, or decode into a typed struct
var result map[string]json.Number
dec := json.NewDecoder(strings.NewReader(`{"id": 1234567890123456789}`))
dec.UseNumber()
var m map[string]any
dec.Decode(&m)
id, _ := m["id"].(json.Number).Int64() // Preserves integer precision
```
