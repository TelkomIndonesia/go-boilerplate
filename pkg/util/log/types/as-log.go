package types

import (
	"reflect"

	"github.com/telkomindonesia/go-boilerplate/pkg/util/log"
)

func AsLog(v any) any {
	m := structToMap(v)
	if m != nil {
		return m
	}

	if v, ok := v.(log.Loggable); ok {
		return v
	}

	return v

}

func structToMap(v interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	val := reflect.ValueOf(v)
	t := reflect.TypeOf(v)

	if t.Kind() == reflect.Ptr {
		val = val.Elem()
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	loggable := reflect.TypeOf((*log.Loggable)(nil)).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		fieldValue := val.Field(i)

		switch {
		// logable
		case fieldValue.Type().Implements(loggable):
			result[field.Name] = fieldValue.Interface().(log.Loggable).AsLog()

		// struct
		case fieldValue.Kind() == reflect.Struct:
			result[field.Name] = structToMap(fieldValue.Interface())

		// slice
		case fieldValue.Kind() == reflect.Slice || fieldValue.Kind() == reflect.Array:
			var sliceResult []interface{}
			for j := 0; j < fieldValue.Len(); j++ {
				item := fieldValue.Index(j)
				if item.Type().Implements(loggable) {
					sliceResult = append(sliceResult, item.Interface().(log.Loggable).AsLog())
				} else if item.Kind() == reflect.Struct {
					sliceResult = append(sliceResult, structToMap(item.Interface()))
				} else {
					sliceResult = append(sliceResult, item.Interface())
				}
			}
			result[field.Name] = sliceResult

		// map
		case fieldValue.Kind() == reflect.Map:
			mapResult := make(map[interface{}]interface{})
			for _, key := range fieldValue.MapKeys() {
				mapValue := fieldValue.MapIndex(key)
				if mapValue.Type().Implements(loggable) {
					mapResult[key.Interface()] = mapValue.Interface().(log.Loggable).AsLog()
				} else if mapValue.Kind() == reflect.Struct {
					mapResult[key.Interface()] = structToMap(mapValue.Interface())
				} else {
					mapResult[key.Interface()] = mapValue.Interface()
				}
			}
			result[field.Name] = mapResult

		// all others
		default:
			result[field.Name] = nil
		}
	}

	return result
}
