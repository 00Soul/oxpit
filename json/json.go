package json

import (
	"encoding/json"
	"reflect"
	"strconv"
)

type Field struct {
	Name          string
	flattenFunc   func(interface{}) interface{}
	unflattenFunc func(interface{}) interface{}
}

type Mapping struct {
	structType    reflect.Type
	flattenFunc   func(interface{}) interface{}
	unflattenFunc func(interface{}) interface{}
	fields        map[string]Field
}

type Context struct {
	mappings map[reflect.Type]Mapping
}

var globalContext *Context = nil

func NewContext() *Context {
	c := new(Context)

	c.mappings = make(map[reflect.Type]Mapping)

	return c
}

func getGlobalContext() {
	if globalContext == nil {
		globalContext = NewContext()
	}

	return globalContext
}

func New(t reflect.Type) *Mapping {
	return getGlobalContext().NewMapping(t)
}

func Get(t reflect.Type) *Mapping {
	return getGlobalContext().Get(t)
}

func Del(t reflect.Type) {
	getGlobalContext().Del(t)
}

func (c *Context) New(t reflect.Type) *Mapping {
	m := new(Mapping)
	m.structType = t
	m.fields = make(map[string]Field)

	c.mappings[t] = m

	return m
}

func (c Context) Get(t reflect.Type) *Mapping {
	m, found := c.mappings[t]
	if found {
		return &m
	} else {
		return nil
	}
}

func (c Context) Del(t reflect.Type) {
	delete(c.mappings, t)
}

func (m *Mapping) Field(sf reflect.StructField) *Field {
	f, found := m.fields[sf.Name]
	if found {
		return &f
	} else {
		return nil
	}
}

func (m *Mapping) FlattenFunc(fn func(interface{}) interface{}) *Mapping {
	m.flattenFunc = fn

	return m
}

func (m *Mapping) UnflattenFunc(fn func(interface{}) interface{}) *Mapping {
	m.unflattenFunc = fn

	return m
}

func (f *Field) Name(name string) *Field {
	f.Name = name

	return f
}

func (f *Field) FlattenFunc(fn func(interface{}) ([]byte, error)) *Field {
	f.flattenFunc = fn

	return f
}

func (f *Field) UnflattenFunc(fn func([]byte, interface{}) error) *Field {
	f.unflattenFunc = fn

	return f
}

func Marshal(v interface{}) ([]byte, error) {
	return getGlobalContext().Marshal(v)
}

func Unmarshal(data []byte, v interface{}) error {
	return getGlobalContext().Unmarshal(data, v)
}

func (c Context) Marshal(i interface{}) ([]byte, error) {
	var v interface{}

	t := reflect.TypeOf(i)

	switch t.Kind() {
	case reflect.Struct:
		v = c.toMap(i)
	case reflect.Slice:
		v = c.toSlice(i)
	default:
		mapping, found := c.mappings[t]
		if found && mapping.flattenFunc != nil {
			v = mapping.flattenFunc(i)
		} else {
			v = i
		}
	}

	return json.Marshal(v)
}

func (c Context) Unmarshal(data []byte, i interface{}) error {
	var v interface{}

	err = json.Unmarshal(data, &v)
	obj := c.unflatten(v, reflect.TypeOf(i))
	&i = &obj

	return err
}

func (c Context) unflatten(i interface{}, t reflect.Type) interface{} {
	v := reflect.New(t).Elem().Interface()

	mapping, found := c.mappings[t]

	if found && mapping.unflattenFunc != nil {
		v = mapping.unflattenFunc(i)
	} else if m, isMap := i.(map[string]interface{}); found && isMap && t.Kind() == reflect.Struct {
		v = c.fromMap(m, t)
	} else if slice, isSlice := i.([]interface{}); isSlice && t.Kind() == reflect.Slice {
		v = c.fromSlice(slice, t)
	} else {
		err := json.Unmarshal([]byte(i), &v)
	}

	return v
}

func (c Context) toSlice(i interface{}) []interface{} {
	slice := make([]interface{}, 0, 1)

	islice, ok := i.([]interface{})
	if ok {
		for _, item := range islice {
			bytes, err := c.Marshal(item)
			if err == nil {
				slice = append(slice, bytes)
			}
		}
	}

	return slice
}

func (c Context) fromSlice(i interface{}) interface{} {
	slice := make([]interface{}, 0, 1)

	islice, ok := i.([]interface{})
	if ok {
		for _, item := range islice {
			slice = append(slice, c.unflatten(item))
		}
	}

	return slice
}

func (c Context) toMap(i interface{}) map[string][]byte {
	m := make(map[string][]byte)

	v := reflect.ValueOf(i)
	mapping, found := c.mappings[v.Type()]

	for n := 0; n < v.NumField(); n++ {
		var field Field
		var fieldFound bool

		name := v.Type().Field(n).Name

		if found {
			field, fieldFound = mapping.fields[name]
			name := mapping.fields[name].Name
		}

		var bytes []bytes
		var err error

		fv := v.Field(n).Interface()

		if fieldFound && field.flattenFunc != nil {
			bytes, err = json.Marshal(field.flattenFunc(fv))
		} else {
			bytes, err = c.Marshal(fv)
		}

		m[name] = bytes
	}

	return m
}

func (c Context) fromMap(m map[string]interface{}, t reflect.Type) reflect.Value {
	p := reflect.New(t)
	v := p.Elem()

	mapping, found := c.mappings[v.Type()]

	for n := 0; n < v.NumField(); n++ {
		var field Field
		var fieldFound bool

		name := v.Type().Field(n).Name

		if found {
			field, fieldFound = mapping.fields[name]
			name := mapping.fields[name].Name
		}

		fv := v.Field(n).Interface()
		if fieldFound && field.unflattenFunc != nil {
			v.Field(n).Set(field.unflattenFunc(m[name]))
		} else {
			bytes, err = c.Marshal(fv)
		}

		m[name] = bytes
	}

	return p
}
