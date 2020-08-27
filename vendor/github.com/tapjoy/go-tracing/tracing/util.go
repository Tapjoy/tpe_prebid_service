package tracing

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/key"
)

type ResponseWriter struct {
	// Wrapped is not embedded to prevent ResponseWriter from directly
	// fulfilling the http.ResponseWriter interface. Wrapping in this
	// way would obscure optional http.ResponseWriter interfaces.
	Wrapped http.ResponseWriter
	Status  int
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	var rw ResponseWriter

	rw.Wrapped = httpsnoop.Wrap(w, httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(code int) {
				// The first call to WriteHeader sends the response header.
				// Any subsequent calls are invalid. Only record the first
				// code written.
				if rw.Status == 0 {
					rw.Status = code
				}
				next(code)
			}
		},
	})

	return &rw
}

func getKeyValues(keyName string, v interface{}) []core.KeyValue {
	result := make([]core.KeyValue, 0)

	vType := reflect.TypeOf(v)
	if vType == nil {
		return result
	}
	val := reflect.ValueOf(v)

	switch vType.Kind() {
	case reflect.Bool:
		result = append(result, key.Bool(keyName, val.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = append(result, key.Int64(keyName, val.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result = append(result, key.Uint64(keyName, val.Uint()))
	case reflect.Float32, reflect.Float64:
		result = append(result, key.Float64(keyName, val.Float()))
	case reflect.String:
		result = append(result, key.String(keyName, val.String()))
	case reflect.Struct:
		result = append(result, addStruct(keyName, val.Interface())...)
	case reflect.Slice, reflect.Array:
		result = append(result, addArray(keyName, val.Interface()))
	case reflect.Map:
		result = append(result, addMap(keyName, val.Interface())...)
	case reflect.Ptr:
		// recurse
		result = append(result, getKeyValues(keyName, val.Interface())...)
	}
	return result
}

func addStruct(keyPrefix string, s interface{}) []core.KeyValue {
	result := make([]core.KeyValue, 0)
	// TODO should we handle embedded structs differently from other deep structs?
	sType := reflect.TypeOf(s)
	sVal := reflect.ValueOf(s)
	// Iterate through the fields, adding each.
	for i := 0; i < sType.NumField(); i++ {
		fieldInfo := sType.Field(i)
		if fieldInfo.PkgPath != "" {
			// skipping unexported field in the struct
			continue
		}

		var fName string
		fTag := fieldInfo.Tag.Get("json")
		if fTag != "" {
			if fTag == "-" {
				// skip this field
				continue
			}
			// slice off options
			if idx := strings.Index(fTag, ","); idx != -1 {
				options := fTag[idx:]
				fTag = fTag[:idx]
				if strings.Contains(options, "omitempty") && isEmptyValue(sVal.Field(i)) {
					// skip empty values if omitempty option is set
					continue
				}
			}
			fName = fTag
		} else {
			fName = fieldInfo.Name
		}
		result = append(result, getKeyValues(keyPrefix+"."+fName, sVal.Field(i).Interface())...)
	}
	return result
}

func addMap(keyPrefix string, m interface{}) []core.KeyValue {
	result := make([]core.KeyValue, 0)

	mVal := reflect.ValueOf(m)
	mKeys := mVal.MapKeys()
	for _, key := range mKeys {
		// get a string representation of key
		var keyStr string
		switch key.Type().Kind() {
		case reflect.String:
			keyStr = key.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
			reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
			reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Complex64,
			reflect.Complex128:
			keyStr = fmt.Sprintf("%v", key.Interface())
		default:
			// skipping unknown
			continue
		}
		result = append(result, getKeyValues(keyPrefix+"."+keyStr, mVal.MapIndex(key).Interface())...)
	}
	return result
}

func addArray(k string, v interface{}) core.KeyValue {
	result := "["
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Array || val.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			result += fmt.Sprintf("%v", val.Index(i))
			if i != val.Len()-1 {
				result += ","
			}
		}
	}
	result += "]"
	return key.String(k, result)
}

// Helper lifted from Go stdlib encoding/json
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
