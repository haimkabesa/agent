package utils

import (
	"fmt"

	om "github.com/wk8/go-ordered-map"
)

type KeyVal = om.OrderedMap

func NewKeyVal() *KeyVal {
	return om.New()
}

func KeyValFromMap(m map[string]string) *KeyVal {
	kv := NewKeyVal()
	for k, v := range m {
		KeyValAdd(kv, k, v)
	}
	return kv
}

func MergeKeyVal(target *KeyVal, source *KeyVal) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(el.Key, el.Value)
	}
}

func MergeKeyValWithPrefix(target *KeyVal, source *KeyVal, prefix string) {
	for el := source.Oldest(); el != nil; el = el.Next() {
		target.Set(fmt.Sprintf("%s%s", prefix, el.Key), el.Value)
	}
}

func KeyValAdd(kv *KeyVal, key string, value string) {
	if len(value) > 0 {
		kv.Set(key, value)
	}
}

func KeyValToInterfaceSlice(kv *KeyVal) []interface{} {
	slice := make([]interface{}, kv.Len()*2)
	idx := 0
	for el := kv.Oldest(); el != nil; el = el.Next() {
		slice[idx] = el.Key
		idx += 1
		slice[idx] = el.Value
		idx += 1
	}
	return slice
}

func KeyValToInterfaceMap(kv *KeyVal) map[string]interface{} {
	retv := make(map[string]interface{})
	for el := kv.Oldest(); el != nil; el = el.Next() {
		retv[fmt.Sprint(el.Key)] = el.Value
	}
	return retv
}
