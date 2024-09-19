package apis

import (
	"github.com/EmilyShepherd/k8s-client-go/types"
)

func Matches[T any, PT types.Object[T]](namespace string, selectors []types.LabelSelector, item PT) bool {
	if namespace != "" && namespace != item.GetNamespace() {
		return false
	}
	return LabelMatch(selectors, item.GetLabels())
}

func LabelMatch(selectors []types.LabelSelector, labels map[string]string) bool {
	for _, selector := range selectors {
		value, exists := labels[selector.Label]
		if !exists {
			return false
		}
		switch selector.Operator {
		case types.Equals:
			if value != selector.Value {
				return false
			}
		case types.NotEquals:
			if value == selector.Value {
				return false
			}
		case types.LessThan:
			if value >= selector.Value {
				return false
			}
		case types.GreaterThan:
			if value <= selector.Value {
				return false
			}
		}
	}

	return true
}
