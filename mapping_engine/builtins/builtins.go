// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package builtins contains function definitions and implementation for built-in mapping functions.
package builtins

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/healthcare-data-harmonization/mapping_engine/projector" /* copybara-comment: projector */
	"github.com/GoogleCloudPlatform/healthcare-data-harmonization/mapping_engine/types" /* copybara-comment: types */
	"github.com/GoogleCloudPlatform/healthcare-data-harmonization/mapping_engine/util/jsonutil" /* copybara-comment: jsonutil */
	"github.com/google/go-cmp/cmp" /* copybara-comment: cmp */
	"bitbucket.org/creachadair/stringset" /* copybara-comment: stringset */
	"github.com/google/uuid" /* copybara-comment: uuid */
)

// When adding a built-in, remember to add it to the map below with its name as the key.
var builtinFunctions = map[string]interface{}{
	// Arithmetic
	"$Div": Div,
	"$Mod": Mod,
	"$Mul": Mul,
	"$Sub": Sub,
	"$Sum": Sum,

	// Collections
	"$Flatten":        Flatten,
	"$ListCat":        ListCat,
	"$ListLen":        ListLen,
	"$ListOf":         ListOf,
	"$SortAndTakeTop": SortAndTakeTop,
	"$UnionBy":        UnionBy,
	"$UnnestArrays":   UnnestArrays,

	// Date/Time
	"$CurrentTime":   CurrentTime,
	"$ParseTime":     ParseTime,
	"$ParseUnixTime": ParseUnixTime,
	"$ReformatTime":  ReformatTime,
	"$SplitTime":     SplitTime,

	// Data operations
	"$Hash":      Hash,
	"$IsNil":     IsNil,
	"$IsNotNil":  IsNotNil,
	"$MergeJSON": MergeJSON,
	"$UUID":      UUID,

	// Debugging
	"$DebugString": DebugString,
	"$Void":        Void,

	// Logic
	"$And": And,
	"$Eq":  Eq,
	"$Gt":  Gt,
	"$GtEq": GtEq,
	"$Lt":  Lt,
	"$LtEq": LtEq,
	"$NEq": NEq,
	"$Not": Not,
	"$Or":  Or,

	// Strings
	"$ParseFloat": ParseFloat,
	"$ParseInt":   ParseInt,
	"$StrCat":     StrCat,
	"$StrFmt":     StrFmt,
	"$StrJoin":    StrJoin,
	"$StrSplit":   StrSplit,
	"$ToLower":    ToLower,
	"$ToUpper":    ToUpper,
}

const (
	defaultTimeFormat = "2006-01-02 03:04:05"
)

// RegisterAll registers all built-ins declared in the built-ins maps. This will wrap the functions
// into types.Projectors using projector.FromFunction.
func RegisterAll(r *types.Registry) error {
	for name, fn := range builtinFunctions {
		proj, err := projector.FromFunction(fn, name)
		if err != nil {
			return fmt.Errorf("failed to create projector from built-in %s: %v", name, err)
		}

		if err = r.RegisterProjector(name, proj); err != nil {
			return fmt.Errorf("failed to register built-in %s: %v", name, err)
		}
	}

	return nil
}

// Although arguments and types can vary, all projectors, including built-ins must return
// (jsonutil.JSONToken, error). The first return value can be any type assignable to
// jsonutil.JSONToken. For predicates that must return a boolean (jsonutil.JSONBool), the type
// will be checked/enforced at runtime.

// Div divides the first argument by the second.
func Div(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONNum, error) {
	return l / r, nil
}

// Mod returns the remainder of dividing the first argument by the second.
func Mod(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONNum, error) {
	res := math.Mod(float64(l), float64(r))
	if math.IsNaN(res) {
		return -1, errors.New("modulo operation returned NaN")
	}
	return jsonutil.JSONNum(res), nil
}

// Mul multiplies together all given arguments. Returns 0 if nothing given.
func Mul(operands ...jsonutil.JSONNum) (jsonutil.JSONNum, error) {
	if len(operands) == 0 {
		return 0, nil
	}

	var res jsonutil.JSONNum = 1
	for _, n := range operands {
		res *= n
	}
	return res, nil
}

// Sub subtracts the second argument from the first.
func Sub(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONNum, error) {
	return l - r, nil
}

// Sum adds up all given values.
func Sum(operands ...jsonutil.JSONNum) (jsonutil.JSONNum, error) {
	var res jsonutil.JSONNum
	for _, n := range operands {
		res += n
	}
	return res, nil
}

// Flatten turns a nested array of arrays (of any depth) into a single array.
// Item ordering is preserved, depth first.
func Flatten(array jsonutil.JSONArr) (jsonutil.JSONArr, error) {
	// This needs to always return an empty array, not a nil value. Nil values
	// may cause NPE down the line.
	res := make(jsonutil.JSONArr, 0)

	for _, item := range array {
		if subArr, ok := item.(jsonutil.JSONArr); ok {
			flat, err := Flatten(subArr)
			if err != nil {
				return nil, err
			}

			res = append(res, flat...)
		} else {
			res = append(res, item)
		}
	}

	return res, nil
}

// ListCat concatenates all given arrays into one array.
func ListCat(args ...jsonutil.JSONArr) (jsonutil.JSONArr, error) {
	if len(args) == 0 {
		return jsonutil.JSONArr{}, nil
	}
	if len(args) == 1 {
		return args[0], nil
	}

	var cat jsonutil.JSONArr
	for _, a := range args {
		cat = append(cat, a...)
	}

	return cat, nil
}

// ListLen finds the length of the array.
func ListLen(in jsonutil.JSONArr) (jsonutil.JSONNum, error) {
	return jsonutil.JSONNum(len(in)), nil
}

// ListOf creates a list of the given tokens.
func ListOf(args ...jsonutil.JSONToken) (jsonutil.JSONArr, error) {
	return jsonutil.JSONArr(args), nil
}

// SortAndTakeTop sorts the elements in the array by the key in the specified direction and returns the top element.
func SortAndTakeTop(arr jsonutil.JSONArr, key jsonutil.JSONStr, desc jsonutil.JSONBool) (jsonutil.JSONToken, error) {
	if len(arr) == 0 {
		return nil, nil
	}
	if len(arr) == 1 {
		return arr[0], nil
	}

	tm := map[string]jsonutil.JSONToken{}
	var keys []string
	for _, t := range arr {
		k, err := jsonutil.GetField(t, string(key))
		if err != nil {
			return nil, err
		}
		kstr := fmt.Sprintf("%v", k)
		tm[kstr] = t
		keys = append(keys, kstr)
	}

	sort.Strings(keys)
	if desc {
		return tm[keys[len(keys)-1]], nil
	}
	return tm[keys[0]], nil
}

// UnionBy unions the items in the given array by the given keys, such that each item
// in the resulting array has a unique combination of those keys. The items in the resulting
// array are ordered deterministically (given Union(x, y, z) and Union(z, x, x, y), resulting
// arrays will be the same).
func UnionBy(items jsonutil.JSONArr, keys ...jsonutil.JSONStr) (jsonutil.JSONArr, error) {
	set := make(map[jsonutil.JSONStr]jsonutil.JSONToken)
	var orderedKeys []jsonutil.JSONStr

	for _, i := range items {
		var key jsonutil.JSONStr

		for _, k := range keys {
			v, err := jsonutil.GetField(i, string(k))
			if err != nil {
				return nil, err
			}

			h, err := Hash(v)
			if err != nil {
				return nil, err
			}

			key += h
		}

		if _, ok := set[key]; !ok {
			orderedKeys = append(orderedKeys, key)
			set[key] = i
		}
	}

	sort.Slice(orderedKeys, func(i int, j int) bool {
		return orderedKeys[i] < orderedKeys[j]
	})

	var arr jsonutil.JSONArr

	for _, k := range orderedKeys {
		arr = append(arr, set[k])
	}

	return arr, nil
}

// UnnestArrays takes a json object in the form {"key1": [{}...], "key2": {}}
// and returns an unnested array in the form [{"k": "key1", "v": {}} ...].
// If the value of a key is an object, it simply returns that object. The
// output is sorted by the keys, and the array ordering is preserved.
func UnnestArrays(c jsonutil.JSONContainer) (jsonutil.JSONArr, error) {
	var out jsonutil.JSONArr

	var keys []string
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var kstr jsonutil.JSONToken = jsonutil.JSONStr(k)

		arr, ok := (*c[k]).(jsonutil.JSONArr)
		if !ok {
			kv := jsonutil.JSONContainer{
				"k": &kstr,
				"v": c[k],
			}
			out = append(out, kv)
			continue
		}

		for _, i := range arr {
			vTkn := i
			kv := jsonutil.JSONContainer{
				"k": &kstr,
				"v": &vTkn,
			}
			out = append(out, kv)
		}
	}

	return out, nil
}

// CurrentTime returns the current time based on the Go func time.Now
// (https://golang.org/pkg/time/#Now). The function accepts a time format layout
// (https://golang.org/pkg/time/#Time.Format) and an IANA formatted time zone
// string (https://www.iana.org/time-zones). A string representing the current
// time is returned. A default layout of '2006-01-02 03:04:05'and a default
// time zone of 'UTC' will be used if not provided.
func CurrentTime(format, tz jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	if len(format) == 0 {
		format = defaultTimeFormat
	}
	tm := time.Now().UTC()
	loc, err := time.LoadLocation(string(tz))
	if err != nil {
		return jsonutil.JSONStr(""), err
	}
	outputTime := tm.In(loc).Format(string(format))
	return jsonutil.JSONStr(outputTime), nil
}

// ParseTime uses a Go time-format to convert date into an ISO-formatted (JavaScript) date time.
// TODO: Untie from Go format.
func ParseTime(format, date jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	return ReformatTime(format, date, time.RFC3339Nano)
}

func parseTime(format, date jsonutil.JSONStr) (time.Time, error) {
	if len(date) == 0 {
		return time.Time{}, nil
	}

	isoDate, err := time.Parse(string(format), string(date))
	if err != nil {
		return time.Time{}, err
	}
	return isoDate, nil
}

// ParseUnixTime parses a unit and a unix timestamp into an ISO-formatted (JavaScript) date time.
func ParseUnixTime(unit jsonutil.JSONStr, ts jsonutil.JSONNum, format, tz jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	sec := int64(ts)
	ns := int64(0)
	switch strings.ToLower(string(unit)) {
	case "s":
		// Do nothing.
	case "ms":
		ns = sec * int64(time.Millisecond)
		sec = 0
	case "us":
		ns = sec * int64(time.Microsecond)
		sec = 0
	case "ns":
		ns = sec
		sec = 0
	default:
		return jsonutil.JSONStr(""), fmt.Errorf("unsupported unit %v, supported units are s, ms, us, ns", unit)
	}
	tm := time.Unix(sec, ns)
	loc, err := time.LoadLocation(string(tz))
	if err != nil {
		return jsonutil.JSONStr(""), fmt.Errorf("unsupported timezone %v", tz)
	}
	tm = tm.In(loc)
	return jsonutil.JSONStr(tm.Format(string(format))), nil
}

// ReformatTime uses a Go time-format to convert date into another Go time-formatted date time.
// TODO: Untie from Go format.
func ReformatTime(inFormat, date, outFormat jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	isoDate, err := parseTime(inFormat, date)
	if err != nil {
		return jsonutil.JSONStr(""), err
	}
	if isoDate.IsZero() {
		return jsonutil.JSONStr(""), nil
	}
	return jsonutil.JSONStr(isoDate.Format(string(outFormat))), nil
}

// SplitTime splits a time string into components based on the Go time-format
// (https://golang.org/pkg/time/#Time.Format) provided.
// An array with all components (year, month, day, hour, minute, second and
// nanosecond) will be returned.
func SplitTime(format, date jsonutil.JSONStr) (jsonutil.JSONArr, error) {
	d, err := parseTime(format, date)
	if err != nil {
		return jsonutil.JSONArr([]jsonutil.JSONToken{}), err
	}
	c := []jsonutil.JSONToken{
		jsonutil.JSONStr(strconv.Itoa(d.Year())),
		jsonutil.JSONStr(strconv.Itoa(int(d.Month()))),
		jsonutil.JSONStr(strconv.Itoa(d.Day())),
		jsonutil.JSONStr(strconv.Itoa(d.Hour())),
		jsonutil.JSONStr(strconv.Itoa(d.Minute())),
		jsonutil.JSONStr(strconv.Itoa(d.Second())),
		jsonutil.JSONStr(strconv.Itoa(d.Nanosecond())),
	}
	return jsonutil.JSONArr(c), nil
}

// Hash converts the given item into a hash. Key order is not considered (array item order is).
// This is not cryptographically secure, and is not to be used for secure hashing.
func Hash(obj jsonutil.JSONToken) (jsonutil.JSONStr, error) {
	h, err := jsonutil.Hash(obj, false)
	if err != nil {
		return "", err
	}
	return jsonutil.JSONStr(hex.EncodeToString(h)), nil
}

// IsNil returns true iff the given object is nil or empty.
func IsNil(object jsonutil.JSONToken) (jsonutil.JSONBool, error) {
	switch t := object.(type) {
	case jsonutil.JSONStr:
		return len(t) == 0, nil
	case jsonutil.JSONArr:
		return len(t) == 0, nil
	case jsonutil.JSONContainer:
		return len(t) == 0, nil
	case nil:
		return true, nil
	}

	return false, nil
}

// IsNotNil returns true iff the given object is not nil or empty.
func IsNotNil(object jsonutil.JSONToken) (jsonutil.JSONBool, error) {
	isNil, err := IsNil(object)
	return !isNil, err
}

// MergeJSON merges the JSONTokens into one JSON object by repeatedly calling the merge
// function. This overwrites single fields and concatenates array fields (unless
// overwriteArrays is true, in which case arrays are overwritten).
func MergeJSON(arr jsonutil.JSONArr, overwriteArrays jsonutil.JSONBool) (jsonutil.JSONToken, error) {
	var out jsonutil.JSONToken
	for _, t := range arr {
		if out == nil {
			out = jsonutil.Deepcopy(t)
			continue
		}
		err := jsonutil.Merge(t, &out, false, bool(overwriteArrays))
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UUID generates a UUID.
func UUID() (jsonutil.JSONStr, error) {
	return jsonutil.JSONStr(uuid.New().String()), nil
}

// DebugString converts the JSON element to a string representation using Go's "%v" format.
func DebugString(t jsonutil.JSONToken) (jsonutil.JSONStr, error) {
	return jsonutil.JSONStr(fmt.Sprintf("%v", t)), nil
}

// Void returns nil given any inputs. You non-nil into the Void, the Void nils back.
func Void(_ ...jsonutil.JSONToken) (jsonutil.JSONToken, error) {
	return nil, nil
}

// And is a logical AND of all given arguments.
func And(args ...jsonutil.JSONBool) (jsonutil.JSONBool, error) {
	if len(args) == 0 {
		return false, nil
	}

	for _, a := range args {
		if !a {
			return false, nil
		}
	}

	return true, nil
}

// Eq returns true iff all given arguments are equal.
func Eq(args ...jsonutil.JSONToken) (jsonutil.JSONBool, error) {
	if len(args) < 2 {
		return true, nil
	}

	for _, arg := range args[1:] {
		if !cmp.Equal(arg, args[0]) {
			return false, nil
		}
	}

	return true, nil
}

// Gt returns true iff the first argument is greater than the second.
func Gt(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONBool, error) {
	return l > r, nil
}

// GtEq returns true iff the first argument is greater than or equal to the second.
func GtEq(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONBool, error) {
	return l >= r, nil
}

// Lt returns true iff the first argument is less than the second.
func Lt(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONBool, error) {
	return l < r, nil
}

// LtEq returns true iff the first argument is less than or equal to the second.
func LtEq(l jsonutil.JSONNum, r jsonutil.JSONNum) (jsonutil.JSONBool, error) {
	return l <= r, nil
}

// NEq returns true iff all given arguments are different.
func NEq(args ...jsonutil.JSONToken) (jsonutil.JSONBool, error) {
	if len(args) < 2 {
		return true, nil
	}

	hashSet := stringset.NewSize(len(args))
	for _, a := range args {
		h, err := Hash(a)
		if err != nil {
			return false, err
		}

		if !hashSet.Add(string(h)) {
			return false, nil
		}
	}

	return true, nil
}

// Not returns true iff the given value is false.
func Not(v jsonutil.JSONBool) (jsonutil.JSONBool, error) {
	return !v, nil
}

// Or is a logical OR of all given arguments.
func Or(args ...jsonutil.JSONBool) (jsonutil.JSONBool, error) {
	for _, a := range args {
		if a {
			return true, nil
		}
	}

	return false, nil
}

// ParseFloat parses a string into a float.
func ParseFloat(str jsonutil.JSONStr) (jsonutil.JSONNum, error) {
	f, err := strconv.ParseFloat(string(str), 64)
	if err != nil {
		return 0, err
	}
	return jsonutil.JSONNum(f), nil
}

// ParseInt parses a string into an int.
func ParseInt(str jsonutil.JSONStr) (jsonutil.JSONNum, error) {
	i, err := strconv.Atoi(string(str))
	if err != nil {
		return -1, err
	}
	return jsonutil.JSONNum(i), nil
}

// StrCat joins the input strings with the separator.
func StrCat(args ...jsonutil.JSONToken) (jsonutil.JSONStr, error) {
	return StrJoin(jsonutil.JSONStr(""), args...)
}

// StrFmt formats the given item using the given Go format specifier.
func StrFmt(format jsonutil.JSONStr, item jsonutil.JSONToken) (jsonutil.JSONStr, error) {
	// This cast avoids formatting issues with numbers (since JSONNum is not detected as a number by the formatter)
	if numItem, ok := item.(jsonutil.JSONNum); ok {
		fmtSpec := format[strings.Index(string(format), "%")+1]
		if strings.Contains("bcdoqxXU", string(fmtSpec)) {
			return jsonutil.JSONStr(fmt.Sprintf(string(format), int(numItem))), nil
		}
		return jsonutil.JSONStr(fmt.Sprintf(string(format), float64(numItem))), nil
	}
	return jsonutil.JSONStr(fmt.Sprintf(string(format), item)), nil
}

// StrJoin concatenates the input strings.
func StrJoin(sep jsonutil.JSONStr, args ...jsonutil.JSONToken) (jsonutil.JSONStr, error) {
	var o []string
	for _, token := range args {
		if token != nil {
			o = append(o, fmt.Sprintf("%v", token))
		}
	}
	return jsonutil.JSONStr(strings.Join(o, string(sep))), nil
}

// StrSplit splits a string by the separator and ignores empty entries.
func StrSplit(str jsonutil.JSONStr, sep jsonutil.JSONStr) (jsonutil.JSONArr, error) {
	outs := strings.Split(string(str), string(sep))
	var res jsonutil.JSONArr
	for _, out := range outs {
		val := strings.TrimSpace(out)
		if len(val) == 0 {
			continue
		}
		res = append(res, jsonutil.JSONStr(val))
	}
	return res, nil
}

// ToLower uses Go's builtin strings.ToLower to convert the given string with all unicode
// characters mapped to their lowercase.
func ToLower(str jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	return jsonutil.JSONStr(strings.ToLower(string(str))), nil
}

// ToUpper uses Go's builtin strings.ToUpper to convert the given string with all unicode
// characters mapped to their uppercase.
func ToUpper(str jsonutil.JSONStr) (jsonutil.JSONStr, error) {
	return jsonutil.JSONStr(strings.ToUpper(string(str))), nil
}
