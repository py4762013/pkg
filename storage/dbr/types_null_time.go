// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbr

import (
	"fmt"
	"time"

	"github.com/corestoreio/errors"
)

func (a NullTime) toIFace(args []interface{}) []interface{} {
	if a.Valid {
		return append(args, a.Time)
	}
	return append(args, nil)
}

func (a NullTime) writeTo(w queryWriter, _ int) error {
	if a.Valid {
		dialect.EscapeTime(w, a.Time)
		return nil
	}
	_, err := w.WriteString(sqlStrNull)
	return err
}

func (a NullTime) len() int { return 1 }

// MakeNullTime creates a new NullTime. Setting the second optional argument to
// false, the string will not be valid anymore, hence NULL. NullTime implements
// interface Argument.
func MakeNullTime(t time.Time, valid ...bool) NullTime {
	v := true
	if len(valid) == 1 {
		v = valid[0]
	}
	return NullTime{
		Time:  t,
		Valid: v,
	}
}

// GoString prints an optimized Go representation.
func (a NullTime) GoString() string {
	if !a.Valid {
		return "dbr.NullTime{}"
	}
	return fmt.Sprintf("dbr.MakeNullTime(time.Unix(%d,%d)", a.Time.Unix(), a.Time.Nanosecond())
}

// MarshalJSON implements json.Marshaler.
// It will encode null if this time is null.
func (a NullTime) MarshalJSON() ([]byte, error) {
	if !a.Valid {
		return []byte("null"), nil
	}
	return a.Time.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports string, object (e.g. pq.NullTime and friends)
// and null input.
func (a *NullTime) UnmarshalJSON(data []byte) error {
	var err error
	var v interface{}
	if err = JSONUnMarshalFn(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		err = a.Time.UnmarshalJSON(data)
	case map[string]interface{}:
		ti, tiOK := x["Time"].(string)
		valid, validOK := x["Valid"].(bool)
		if !tiOK || !validOK {
			return errors.NewNotValidf(`[dbr] json: unmarshalling object into Go value of type dbr.NullTime requires key "Time" to be of type string and key "Valid" to be of type bool; found %T and %T, respectively`, x["Time"], x["Valid"])
		}
		err = a.Time.UnmarshalText([]byte(ti))
		a.Valid = valid
		return err
	case nil:
		a.Valid = false
		return nil
	default:
		err = errors.NewNotValidf("[dbr] json: cannot unmarshal %#v into Go value of type dbr.NullTime", v)
	}
	a.Valid = err == nil
	return err
}

// MarshalText transforms the time type into a byte slice.
func (a NullTime) MarshalText() ([]byte, error) {
	if !a.Valid {
		return []byte("null"), nil
	}
	return a.Time.MarshalText()
}

// UnmarshalText parses the byte slice to create a time type.
func (a *NullTime) UnmarshalText(text []byte) error {
	str := string(text)
	if str == "" || str == "null" {
		a.Valid = false
		return nil
	}
	if err := a.Time.UnmarshalText(text); err != nil {
		return err
	}
	a.Valid = true
	return nil
}

// SetValid changes this Time's value and sets it to be non-null.
func (a *NullTime) SetValid(v time.Time) {
	a.Time = v
	a.Valid = true
}

// Ptr returns a pointer to this Time's value, or a nil pointer if this Time is null.
func (a NullTime) Ptr() *time.Time {
	if !a.Valid {
		return nil
	}
	return &a.Time
}

// ArgNullTimes adds a nullable Time or a slice of nullable Timess to the
// argument list. Providing no arguments returns a NULL type.
type ArgNullTimes []NullTime

func (a ArgNullTimes) toIFace(args []interface{}) []interface{} {
	for _, s := range a {
		if s.Valid {
			args = append(args, s.Time)
		} else {
			args = append(args, nil)
		}
	}
	return args
}

func (a ArgNullTimes) writeTo(w queryWriter, pos int) error {
	if s := a[pos]; s.Valid {
		dialect.EscapeTime(w, s.Time)
		return nil
	}
	_, err := w.WriteString(sqlStrNull)
	return err
}

func (a ArgNullTimes) len() int {
	return len(a)
}
