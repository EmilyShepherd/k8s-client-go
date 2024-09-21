package util

import (
	"github.com/EmilyShepherd/k8s-client-go/types"
)

func GetKey(namespace, name string) string {
	key := ""
	if namespace != "" {
		key = namespace + "/"
	}

	key += name

	return key
}

func GetKeyForObject[T any, PT types.Object[T]](o PT) string {
	return GetKey(o.GetNamespace(), o.GetName())
}
