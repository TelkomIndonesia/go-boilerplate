package types

import (
	"reflect"

	"github.com/telkomindonesia/go-boilerplate/pkg/log"
)

func AsLog(v any) any {
	return asLogRecurse(v, true)
}

func asLogRecurse(v any, root bool) any {
	loggable := reflect.TypeOf((*log.Loggable)(nil)).Elem()
	value := reflect.ValueOf(v)
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		value = value.Elem()
		t = t.Elem()
	}

	if !value.IsValid() {
		return value
	}

	switch {
	case !root && value.Type().Implements(loggable):
		return value.Interface().(log.Loggable).AsLog()

	case value.Kind() == reflect.Struct:
		result := make(map[string]interface{})
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}

			result[field.Name] = asLogRecurse(value.Field(i).Interface(), false)
		}
		return result

	case value.Kind() == reflect.Slice || value.Kind() == reflect.Array:
		var result = make([]interface{}, 0, value.Len())
		for j := 0; j < value.Len(); j++ {
			item := value.Index(j)
			result = append(result, asLogRecurse(item.Interface(), false))
		}
		return result

	case value.Kind() == reflect.Map:
		result := make(map[interface{}]interface{})
		for _, key := range value.MapKeys() {
			mapValue := value.MapIndex(key)
			result[key.Interface()] = asLogRecurse(mapValue.Interface(), false)
		}
		return result

	default:
		return v
	}
}
