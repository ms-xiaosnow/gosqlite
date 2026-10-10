package sql

import (
	"fmt"
	"math"
	"strings"
)

// AggregateName identifies the bounded global-aggregate profile. MIN/MAX with
// multiple arguments are scalar SQLite functions and are deliberately not
// implemented by this aggregate path.
func AggregateName(name string) bool {
	switch FoldName(name) {
	case "count", "sum", "total", "avg", "min", "max":
		return true
	}
	return false
}

// Aggregate is constant-sized native state, except that MIN/MAX retain one
// ordinary SQL value. Its owner admits operand scratch and clones retained
// TEXT/BLOB bytes before Step, and supplies the statement's shared budget.
// Window inverse operations and DISTINCT are intentionally absent.
type Aggregate struct {
	Name                  string
	count, integer        int64
	sum, correction       float64
	approximate, overflow bool
	value                 any
}

func (a *Aggregate) Step(v any) error {
	switch v.(type) {
	case nil, int64, float64, string, []byte:
	default:
		return &ErrUnsupported{Feature: fmt.Sprintf("%s aggregate value type %T", a.Name, v)}
	}
	if v == nil {
		return nil
	}
	if a.Name == "min" || a.Name == "max" {
		if a.value == nil {
			a.value = v
			return nil
		}
		c, err := Compare(v, a.value)
		if err != nil {
			return err
		}
		if (a.Name == "min" && c < 0) || (a.Name == "max" && c > 0) {
			a.value = v
		}
		return nil
	}
	if a.count == math.MaxInt64 {
		return fmt.Errorf("integer overflow")
	}
	a.count++
	if a.Name == "count" {
		return nil
	}
	n := aggregateNumber(v)
	i, integral := n.(int64)
	if !a.approximate {
		if integral {
			sum := a.integer + i
			if (i > 0 && sum < a.integer) || (i < 0 && sum > a.integer) {
				a.overflow, a.approximate = true, true
				a.startReal(a.integer)
				a.addInteger(i)
			} else {
				a.integer = sum
			}
		} else {
			a.startReal(a.integer)
			a.approximate = true
			a.addReal(real(n))
		}
	} else if integral {
		a.addInteger(i)
	} else {
		// SQLite 3.54.0 clears an earlier integer overflow when a later
		// non-integer numeric type is encountered, even nonnumeric TEXT/BLOB.
		a.overflow = false
		a.addReal(real(n))
	}
	return nil
}

// sqlite3_value_numeric_type converts fully numeric TEXT, preserving a decimal
// point/exponent as REAL. BLOB and partially numeric TEXT use double conversion.
func aggregateNumber(v any) any {
	switch x := v.(type) {
	case int64, float64:
		return x
	case string:
		// SQLite numeric TEXT conversion treats the first embedded NUL as
		// its terminator. Keep BLOB conversion distinct: its numeric prefix
		// is always a REAL input, even when the same bytes spell an integer.
		if n := strings.IndexByte(x, 0); n >= 0 {
			x = x[:n]
		}
		if _, text := numericText(x, true).(string); !text {
			return numericText(x, false)
		}
		return real(numericText(x, false))
	case []byte:
		return real(numericText(string(x), false))
	}
	return float64(0)
}

// Compensated accumulation follows the pinned public SQLite func.c arithmetic
// contract, including split large integers and overflow correction handling.
func (a *Aggregate) startReal(i int64) {
	if i <= -(1<<52) || i >= 1<<52 {
		small := i % 16384
		a.sum, a.correction = float64(i-small), float64(small)
	} else {
		a.sum, a.correction = float64(i), 0
	}
}
func (a *Aggregate) addInteger(i int64) {
	if i <= -(1<<52) || i >= 1<<52 {
		small := i % 16384
		a.addReal(float64(i - small))
		a.addReal(float64(small))
	} else {
		a.addReal(float64(i))
	}
}
func (a *Aggregate) addReal(v float64) {
	old := a.sum
	next := old + v
	if math.Abs(old) > math.Abs(v) {
		a.correction += (old - next) + v
	} else {
		a.correction += (v - next) + old
	}
	a.sum = next
}
func (a *Aggregate) Result() (any, error) {
	switch a.Name {
	case "count":
		return a.count, nil
	case "min", "max":
		return a.value, nil
	}
	if a.count == 0 && a.Name != "total" {
		return nil, nil
	}
	if a.Name == "sum" && !a.approximate {
		return a.integer, nil
	}
	if a.Name == "sum" && a.overflow {
		return nil, fmt.Errorf("integer overflow")
	}
	result := float64(a.integer)
	if a.approximate {
		result = a.sum
		if !math.IsInf(a.correction, 0) && !math.IsNaN(a.correction) {
			result += a.correction
		}
	}
	if a.Name == "avg" {
		result /= float64(a.count)
	}
	if math.IsNaN(result) {
		return nil, nil
	}
	return result, nil
}

// RetainedValue reports the sole MIN/MAX value, allowing its owner to charge
// and replace its backing bytes without coupling scalar SQL to storage quotas.
func (a *Aggregate) RetainedValue() any { return a.value }
