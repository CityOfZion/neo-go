// Code generated by "stringer -type=AttrType -linecomment"; DO NOT EDIT.

package transaction

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[HighPriority-1]
	_ = x[OracleResponseT-17]
}

const (
	_AttrType_name_0 = "HighPriority"
	_AttrType_name_1 = "OracleResponse"
)

func (i AttrType) String() string {
	switch {
	case i == 1:
		return _AttrType_name_0
	case i == 17:
		return _AttrType_name_1
	default:
		return "AttrType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
