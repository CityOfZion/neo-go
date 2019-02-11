package rpc

import (
	"encoding/json"
	"net/http"
)

type (
	// Params represent the JSON-RPC params.
	Params []Param
)

// UnmarshalJSON implements the Unmarshaller
// interface.
func (p *Params) UnmarshalJSON(data []byte) error {
	var params []interface{}

	err := json.Unmarshal(data, &params)
	if err != nil {
		return err
	}

	for i := 0; i < len(params); i++ {
		param := Param{
			RawValue: params[i],
		}

		switch val := params[i].(type) {
		case string:
			param.StringVal = val
			param.Type = "string"

		case float64:
			newVal, _ := params[i].(float64)
			param.IntVal = int(newVal)
			param.Type = "number"
		}

		*p = append(*p, param)
	}

	return nil
}

// ValueAt returns the param struct for the given
// index if it exists.
func (p Params) ValueAt(index int) (*Param, bool) {
	if len(p) > index {
		return &p[index], true
	}

	return nil, false
}

// ValueAtAndType returns the param struct at the given index if it
// exists and matches the given type.
func (p Params) ValueAtAndType(index int, valueType string) (*Param, bool) {
	if len(p) > index && valueType == p[index].Type {
		return &p[index], true
	}

	return nil, false
}

func (p Params) Value(index int) (*Param, *Error) {
	if len(p) <= index {
		return nil, newError(-2146233086, http.StatusOK, "Index was out of range. Must be non-negative and less than the size of the collection.\nParameter name: index", "", nil)
	}
	return &p[index], nil
}

func (p Params) ValueWithType(index int, valType string) (*Param, *Error) {
	val, err := p.Value(index)
	if err != nil {
		return nil, err
	} else if val.Type != valType {
		return nil, newError(-2146233033, http.StatusOK, "One of the identified items was in an invalid format.", "", nil)
	}
	return &p[index], nil
}
