// Copyright 2018 PJ Engineering and Business Solutions Pty. Ltd. All rights reserved.

package dataframe

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
)

// SeriesTime is used for series containing time.Time data.
type SeriesTime struct {
	valFormatter ValueToStringFormatter

	lock     sync.RWMutex
	name     string
	values   []*time.Time
	nilCount int
}

// NewSeriesTime creates a new series with the underlying type as time.Time
func NewSeriesTime(name string, init *SeriesInit, vals ...interface{}) *SeriesTime {
	s := &SeriesTime{
		name:     name,
		values:   []*time.Time{},
		nilCount: 0,
	}

	var (
		size     int
		capacity int
	)

	if init != nil {
		size = init.Size
		capacity = init.Capacity
		if size > capacity {
			capacity = size
		}
	}

	s.values = make([]*time.Time, size, capacity)
	s.valFormatter = DefaultValueFormatter

	for idx, v := range vals {
		val := s.valToPointer(v)
		if val == nil {
			s.nilCount++
		}

		if idx < size {
			s.values[idx] = val
		} else {
			s.values = append(s.values, val)
		}
	}

	if len(vals) < size {
		s.nilCount = s.nilCount + size - len(vals)
	}

	return s
}

// Name returns the series name.
func (s *SeriesTime) Name() string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.name
}

// Rename renames the series.
func (s *SeriesTime) Rename(n string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.name = n
}

// Type returns the type of data the series holds.
func (s *SeriesTime) Type() string {
	return "time"
}

// NRows returns how many rows the series contains.
func (s *SeriesTime) NRows(options ...Options) int {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.RLock()
		defer s.lock.RUnlock()
	}

	return len(s.values)
}

// Value returns the value of a particular row.
// The return value could be nil or the concrete type
// the data type held by the series.
// Pointers are never returned.
func (s *SeriesTime) Value(row int, options ...Options) interface{} {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.RLock()
		defer s.lock.RUnlock()
	}

	val := s.values[row]
	if val == nil {
		return nil
	}
	return *val
}

// ValueString returns a string representation of a
// particular row. The string representation is defined
// by the function set in SetValueToStringFormatter.
// By default, a nil value is returned as "NaN".
func (s *SeriesTime) ValueString(row int, options ...Options) string {
	return s.valFormatter(s.Value(row, options...))
}

// Prepend is used to set a value to the beginning of the
// series. val can be a concrete data type or nil. Nil
// represents the absence of a value.
func (s *SeriesTime) Prepend(val interface{}, options ...Options) {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
	}

	// See: https://stackoverflow.com/questions/41914386/what-is-the-mechanism-of-using-append-to-prepend-in-go

	if cap(s.values) > len(s.values) {
		// There is already extra capacity so copy current values by 1 spot
		s.values = s.values[:len(s.values)+1]
		copy(s.values[1:], s.values)
		s.values[0] = s.valToPointer(val)
		return
	}

	// No room, new slice needs to be allocated:
	s.insert(0, val)
}

// Append is used to set a value to the end of the series.
// val can be a concrete data type or nil. Nil represents
// the absence of a value.
func (s *SeriesTime) Append(val interface{}, options ...Options) int {
	var locked bool
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
		locked = true
	}

	row := s.NRows(Options{DontLock: locked})
	s.insert(row, val)
	return row
}

// Insert is used to set a value at an arbitrary row in
// the series. All existing values from that row onwards
// are shifted by 1. val can be a concrete data type or nil.
// Nil represents the absence of a value.
func (s *SeriesTime) Insert(row int, val interface{}, options ...Options) {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
	}

	s.insert(row, val)
}

func (s *SeriesTime) insert(row int, val interface{}) {

	switch V := val.(type) {
	case []time.Time:
		var vals []*time.Time
		for _, v := range V {
			v := v
			vals = append(vals, &v)
		}
		s.values = append(s.values[:row], append(vals, s.values[row:]...)...)
		return
	case []*time.Time:
		for _, v := range V {
			if v == nil {
				s.nilCount++
			}
		}
		s.values = append(s.values[:row], append(V, s.values[row:]...)...)
		return
	}

	s.values = append(s.values, nil)
	copy(s.values[row+1:], s.values[row:])

	v := s.valToPointer(val)
	if v == nil {
		s.nilCount++
	}

	s.values[row] = s.valToPointer(v)
}

// Remove is used to delete the value of a particular row.
func (s *SeriesTime) Remove(row int, options ...Options) {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
	}

	if s.values[row] == nil {
		s.nilCount--
	}

	s.values = append(s.values[:row], s.values[row+1:]...)
}

// Update is used to update the value of a particular row.
// val can be a concrete data type or nil. Nil represents
// the absence of a value.
func (s *SeriesTime) Update(row int, val interface{}, options ...Options) {
	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
	}

	newVal := s.valToPointer(val)

	if s.values[row] == nil && newVal != nil {
		s.nilCount--
	} else if s.values[row] != nil && newVal == nil {
		s.nilCount++
	}

	s.values[row] = newVal
}

func (s *SeriesTime) valToPointer(v interface{}) *time.Time {
	switch val := v.(type) {
	case nil:
		return nil
	case *time.Time:
		if val == nil {
			return nil
		}
		return &[]time.Time{*val}[0]
	case time.Time:
		return &val
	default:
		_ = v.(time.Time) // Intentionally panic
		return nil
	}
}

// SetValueToStringFormatter is used to set a function
// to convert the value of a particular row to a string
// representation.
func (s *SeriesTime) SetValueToStringFormatter(f ValueToStringFormatter) {
	if f == nil {
		s.valFormatter = DefaultValueFormatter
		return
	}
	s.valFormatter = f
}

// Swap is used to swap 2 values based on their row position.
func (s *SeriesTime) Swap(row1, row2 int, options ...Options) {
	if row1 == row2 {
		return
	}

	if len(options) == 0 || (len(options) > 0 && !options[0].DontLock) {
		s.lock.Lock()
		defer s.lock.Unlock()
	}

	s.values[row1], s.values[row2] = s.values[row2], s.values[row1]
}

// IsEqualFunc returns true if a is equal to b.
func (s *SeriesTime) IsEqualFunc(a, b interface{}) bool {

	if a == nil {
		if b == nil {
			return true
		}
		return false
	}

	if b == nil {
		return false
	}
	t1 := a.(time.Time)
	t2 := b.(time.Time)

	return t1.Equal(t2)
}

// IsLessThanFunc returns true if a is less than b.
func (s *SeriesTime) IsLessThanFunc(a, b interface{}) bool {

	if a == nil {
		if b == nil {
			return true
		}
		return true
	}

	if b == nil {
		return false
	}
	t1 := a.(time.Time)
	t2 := b.(time.Time)

	return t1.Before(t2)
}

// Sort will sort the series.
func (s *SeriesTime) Sort(options ...Options) {

	var sortDesc bool

	if len(options) == 0 {
		s.lock.Lock()
		defer s.lock.Unlock()
	} else {
		if !options[0].DontLock {
			s.lock.Lock()
			defer s.lock.Unlock()
		}
		sortDesc = options[0].SortDesc
	}

	sort.SliceStable(s.values, func(i, j int) (ret bool) {
		defer func() {
			if sortDesc {
				ret = !ret
			}
		}()

		if s.values[i] == nil {
			if s.values[j] == nil {
				// both are nil
				return true
			}
			return true
		}

		if s.values[j] == nil {
			// i has value and j is nil
			return false
		}
		// Both are not nil
		ti := *s.values[i]
		tj := *s.values[j]

		return ti.Before(tj)
	})
}

// Lock will lock the Series allowing you to directly manipulate
// the underlying slice with confidence.
func (s *SeriesTime) Lock() {
	s.lock.Lock()
}

// Unlock will unlock the Series that was previously locked.
func (s *SeriesTime) Unlock() {
	s.lock.Unlock()
}

// Copy will create a new copy of the series.
// It is recommended that you lock the Series before attempting
// to Copy.
func (s *SeriesTime) Copy(r ...Range) Series {

	if len(s.values) == 0 {
		return &SeriesTime{
			valFormatter: s.valFormatter,
			name:         s.name,
			values:       []*time.Time{},
			nilCount:     s.nilCount,
		}
	}

	if len(r) == 0 {
		r = append(r, Range{})
	}

	start, end, err := r[0].Limits(len(s.values))
	if err != nil {
		panic(err)
	}

	// Copy slice
	x := s.values[start : end+1]
	newSlice := append(x[:0:0], x...)

	return &SeriesTime{
		valFormatter: s.valFormatter,
		name:         s.name,
		values:       newSlice,
		nilCount:     s.nilCount,
	}
}

// Table will produce the Series in a table.
func (s *SeriesTime) Table(r ...Range) string {

	s.lock.RLock()
	defer s.lock.RUnlock()

	if len(r) == 0 {
		r = append(r, Range{})
	}

	data := [][]string{}

	headers := []string{"", s.name} // row header is blank
	footers := []string{fmt.Sprintf("%dx%d", len(s.values), 1), s.Type()}

	if len(s.values) > 0 {

		start, end, err := r[0].Limits(len(s.values))
		if err != nil {
			panic(err)
		}

		for row := start; row <= end; row++ {
			sVals := []string{fmt.Sprintf("%d:", row), s.ValueString(row, Options{true, false})}
			data = append(data, sVals)
		}

	}

	var buf bytes.Buffer

	table := tablewriter.NewWriter(&buf)
	table.SetHeader(headers)
	for _, v := range data {
		table.Append(v)
	}
	table.SetFooter(footers)
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	table.Render()

	return buf.String()
}

// String implements Stringer interface.
func (s *SeriesTime) String() string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	count := len(s.values)

	out := "[ "

	if count > 6 {
		idx := []int{0, 1, 2, count - 3, count - 2, count - 1}
		for j, row := range idx {
			if j == 3 {
				out = out + "... "
			}
			out = out + s.ValueString(row, Options{true, false}) + " "
		}
		return out + "]"
	}

	for row := range s.values {
		out = out + s.ValueString(row, Options{true, false}) + " "
	}
	return out + "]"
}

// ContainsNil will return whether or not the series contains any nil values.
func (s *SeriesTime) ContainsNil() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.nilCount > 0
}
