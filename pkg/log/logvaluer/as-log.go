package logvaluer

import (
	"reflect"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

func AsLog(v any) any {
	v, _ = asLogRecurse(v, true)
	return v
}

func asLogRecurse(v any, root bool) (any, bool) {
	value := reflect.ValueOf(v)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		value = value.Elem()
		t = t.Elem()
	}

	if !value.IsValid() {
		return value, false
	}

	Loggable := reflect.TypeOf((*log.Valuer)(nil)).Elem()
	switch {
	case !root && value.Type().Implements(Loggable):
		return value.Interface().(log.Valuer).AsLog(), true

	case value.Kind() == reflect.Struct:
		result := make(map[string]interface{})
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}

			v, ok := asLogRecurse(value.Field(i).Interface(), false)
			if ok {
				result[field.Name] = v
			}
		}
		return result, true

	case value.Kind() == reflect.Slice || value.Kind() == reflect.Array:
		var result = make([]interface{}, 0, value.Len())
		for j := 0; j < value.Len(); j++ {
			item := value.Index(j)
			v, ok := asLogRecurse(item.Interface(), false)
			if ok {
				result = append(result, v)

			}
		}
		return result, true

	case value.Kind() == reflect.Map:
		result := make(map[string]interface{})
		for _, key := range value.MapKeys() {
			mapValue := value.MapIndex(key)
			v, ok := asLogRecurse(mapValue.Interface(), false)
			if ok {
				result[key.String()] = v
			}
		}
		return result, true

	case value.Kind() == reflect.Func:
		return nil, false

	default:
		return v, true
	}
}
