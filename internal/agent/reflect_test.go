package agent

import "reflect"

func reflectType[T any]() reflect.Type {
	var zero T
	return reflect.TypeOf(zero)
}
