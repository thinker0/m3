// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package test

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

type Marshaller interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte, interface{}) error
	ID() string
}

var (
	JSONMarshaller Marshaller = simpleMarshaller{
		id:        "json",
		marshal:   json.Marshal,
		unmarshal: json.Unmarshal,
	}

	YAMLMarshaller Marshaller = simpleMarshaller{
		id:        "yaml",
		marshal:   yaml.Marshal,
		unmarshal: yaml.Unmarshal,
	}

	TextMarshaller Marshaller = simpleMarshaller{
		id: "text",
		marshal: func(i interface{}) ([]byte, error) {
			switch m := i.(type) {
			case encoding.TextMarshaler:
				return m.MarshalText()
			default:
				return nil, fmt.Errorf("not an encoding.TextMarshaler")
			}
		},
		unmarshal: func(bytes []byte, i interface{}) error {
			switch m := i.(type) {
			case encoding.TextUnmarshaler:
				return m.UnmarshalText(bytes)
			default:
				return fmt.Errorf("not an encoding.TextUnmarshaler")
			}
		},
	}
)

type simpleMarshaller struct {
	marshal   func(interface{}) ([]byte, error)
	unmarshal func([]byte, interface{}) error
	id        string
}

func (sm simpleMarshaller) Marshal(v interface{}) ([]byte, error) {
	return sm.marshal(v)
}

func (sm simpleMarshaller) Unmarshal(d []byte, v interface{}) error {
	return sm.unmarshal(d, v)
}

func (sm simpleMarshaller) ID() string {
	return sm.id
}

func AssertMarshallingRoundtrips(t *testing.T, marshaller Marshaller, v interface{}) bool {
	d, err := marshaller.Marshal(v)
	if !assert.NoError(t, err) {
		return false
	}
	reconstituted, err := unmarshalIntoNewValueOfType(marshaller, v, d)
	if !assert.NoError(t, err) {
		return false
	}

	return assert.Equal(t, v, reconstituted)
}

func AssertUnmarshals(t *testing.T, marshaller Marshaller, expected interface{}, data []byte) bool {
	unmarshalled, err := unmarshalIntoNewValueOfType(marshaller, expected, data)
	if !assert.NoError(t, err) {
		return false
	}

	return assert.Equal(t, expected, unmarshalled)
}

func AssertMarshals(t *testing.T, marshaller Marshaller, toMarshal interface{}, expectedData []byte) bool {
	marshalled, err := marshaller.Marshal(toMarshal)
	if !assert.NoError(t, err) {
		return false
	}

	return assert.Equal(t, string(expectedData), string(marshalled))
}

// unmarshalIntoNewValueOfType is a helper to unmarshal a new instance of the same type as value
// from data.
func unmarshalIntoNewValueOfType(marshaller Marshaller, value interface{}, data []byte) (interface{}, error) {
	ptrToUnmarshalTarget := reflect.New(reflect.ValueOf(value).Type())
	unmarshalTarget := ptrToUnmarshalTarget.Elem()
	if err := marshaller.Unmarshal(data, ptrToUnmarshalTarget.Interface()); err != nil {
		return nil, err
	}
	return unmarshalTarget.Interface(), nil
}

// Require wraps an Assert call and turns it into a require.
func Require(t *testing.T, b bool) {
	if !b {
		t.FailNow()
	}
}

func TestMarshallersRoundtrip(t *testing.T, examples interface{}, marshallers []Marshaller) {
	for _, m := range marshallers {
		t.Run(m.ID(), func(t *testing.T) {
			v := reflect.ValueOf(examples)
			if v.Type().Kind() != reflect.Slice {
				t.Fatalf("examples must be a slice; got %+v", examples)
			}

			// taken from https://stackoverflow.com/questions/14025833/range-over-interface-which-stores-a-slice
			for i := 0; i < v.Len(); i++ {
				example := v.Index(i).Interface()
				Require(t, AssertMarshallingRoundtrips(t, m, example))
			}
		})

	}
}
