package util

import (
	"reflect"

	"github.com/golang/glog"
)

// Dump object
func Dump(v interface{}) {
	if v == nil {
		return
	}
	s := reflect.ValueOf(v).Elem()
	typeOfT := s.Type()

	glog.V(1).Infof(typeOfT.String())
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		glog.V(1).Infof("%d: %s %s = '%v'", i, typeOfT.Field(i).Name, f.Type(), f.Interface())
	}
}