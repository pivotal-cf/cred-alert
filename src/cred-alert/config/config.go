package config

import "reflect"

func allSet(xs ...string) bool {
	for i := range xs {
		if xs[i] == "" {
			return false
		}
	}

	return true
}

func allBlankOrAllSet(xs ...string) bool {
	var blanks int
	for i := range xs {
		if xs[i] == "" {
			blanks++
		}
	}

	return blanks == len(xs) || blanks == 0
}

// From src/pkg/encoding/json.
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

func merge(dst, src reflect.Value) error {
	if !src.IsValid() {
		// this means the value is the default value,
		// which we don't want to set on dest
		return nil
	}

	switch src.Kind() {
	case reflect.Struct:
		for i, n := 0, dst.NumField(); i < n; i++ {
			err := merge(dst.Field(i), src.Field(i))
			if err != nil {
				return err
			}
		}
	default:
		if dst.CanSet() && !isEmptyValue(src) {
			dst.Set(src)
		}
	}

	return nil
}
