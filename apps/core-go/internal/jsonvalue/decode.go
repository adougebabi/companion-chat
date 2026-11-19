// Package jsonvalue preserves numbers through Core-owned JSON copy boundaries.
package jsonvalue

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// Unmarshal follows encoding/json, retaining json.Number in interface fields
// only when float64 JSON serialization would change the numeric value. Ordinary
// numbers retain their historical float64 type; typed fields and RawMessage
// keep encoding/json's behavior. No field names or user strings are rewritten.
func Unmarshal(data []byte, target any) error {
	if !json.Valid(data) {
		return json.Unmarshal(data, target)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	normalize(reflect.ValueOf(target))
	return nil
}

func normalize(value reflect.Value) {
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			normalize(value.Elem())
		}
	case reflect.Interface:
		if value.IsNil() {
			return
		}
		if number, ok := value.Interface().(json.Number); ok {
			f, err := number.Float64()
			if err == nil && equalNumber(string(number), strconv.FormatFloat(f, 'g', -1, 64)) {
				value.Set(reflect.ValueOf(f))
			}
			return
		}
		normalize(value.Elem())
	case reflect.Map:
		for _, key := range value.MapKeys() {
			item := reflect.New(value.Type().Elem()).Elem()
			item.Set(value.MapIndex(key))
			normalize(item)
			value.SetMapIndex(key, item)
		}
	case reflect.Slice, reflect.Array:
		// Preserve raw JSON bytes exactly, including reference snapshot hashes.
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		for i := 0; i < value.Len(); i++ {
			normalize(value.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanSet() {
				normalize(value.Field(i))
			}
		}
	}
}

func equalNumber(a, b string) bool {
	ad, ae, aok := decimal(a)
	bd, be, bok := decimal(b)
	return aok && bok && ad == bd && ae == be
}

// Compare decimal values without allocating big integers for large exponents.
// This is linear in the JSON literal length, including underflowing numbers.
func decimal(raw string) (string, int64, bool) {
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(raw), "e")
	sign := ""
	if strings.HasPrefix(mantissa, "-") {
		sign = "-"
		mantissa = mantissa[1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "0", 0, true
	}
	var scale int64
	if hasExponent {
		var err error
		scale, err = strconv.ParseInt(exponent, 10, 64)
		if err != nil {
			return "", 0, false
		}
	}
	trimmed := strings.TrimRight(digits, "0")
	delta := int64(len(digits)-len(trimmed)) - int64(len(fraction))
	// Finite nonzero float roundtrips have bounded scales. Treat an integer
	// exponent overflow as non-equivalence and retain the original json.Number.
	if delta > 0 && scale > int64(^uint64(0)>>1)-delta || delta < 0 && scale < (-int64(^uint64(0)>>1)-1)-delta {
		return "", 0, false
	}
	return sign + trimmed, scale + delta, true
}
