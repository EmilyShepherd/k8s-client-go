package util

import (
	"strings"

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

func GetObjectForKey(key string) (string, string) {
	list := strings.Split(key, "/")

	if len(list) == 1 {
		return "", list[0]
	} else {
		return list[0], list[1]
	}
}
