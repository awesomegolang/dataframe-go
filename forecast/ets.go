package forecast

import (
	"context"
	"errors"

	"github.com/bradfitz/iter"
	"github.com/rocketlaunchr/dataframe-go"
)

// SimpleExponentialSmoothing method calculates
// and returns forecast for future m periods
//
//// s - dataframe.SeriesFloat64 object
//
// y - Time series data gotten from s.
// alpha - Exponential smoothing coefficients for level, trend,
//        seasonal components.
// m - Intervals into the future to forecast
//
// https://www.itl.nist.gov/div898/handbook/pmc/section4/pmc431.htm
// newvalue = smoothing * next + (1 - smoothing)*old value
// forecast[i+1] = St[i] + alpha * ϵt,
// where ϵt is the forecast error (actual - forecast) for period i.
func SimpleExponentialSmoothing(ctx context.Context, s *dataframe.SeriesFloat64, α float64, m int, r ...dataframe.Range) (*dataframe.SeriesFloat64, error) {

	if len(r) == 0 {
		r = append(r, dataframe.Range{})
	}

	count := len(s.Values)
	if count == 0 {
		return nil, errors.New("no values in series range")
	}

	start, end, err := r[0].Limits(count)
	if err != nil {
		return nil, err
	}

	// Validation
	if end-start < 1 {
		return nil, errors.New("no values in series range")
	}

	if m <= 0 {
		return nil, errors.New("m must be greater than 0")
	}

	if (α < 0.0) || (α > 1.0) {
		return nil, errors.New("α must be between [0,1]")
	}

	forecast := make([]float64, 0, m)
	var st float64
	for i := start; i < end+1; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		xt := s.Values[i]

		if i == start {
			st = xt
		} else {
			st = α*xt + (1-α)*st
		}
	}

	// Now calculate forecast
	for range iter.N(m) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		st = α*s.Values[end] + (1-α)*st
		forecast = append(forecast, st)
	}

	fdf := dataframe.NewSeriesFloat64("forecast", nil)
	fdf.Values = forecast

	return fdf, nil
}
