package resourcecore

import (
	"strconv"

	"github.com/shopspring/decimal"
)

func CanonicalIntString(v int64) (string, error) { return strconv.FormatInt(v, 10), nil }

func CanonicalDecimalString(s string) (string, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return "", err
	}
	if d.IsZero() {
		return "0", nil
	}
	return d.String(), nil
}

func NewIntegerValue(v int64) Value { return Value{Kind: ValueInteger, Text: strconv.FormatInt(v, 10)} }

func NewDecimalValue(s string) (Value, error) {
	c, err := CanonicalDecimalString(s)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValueDecimal, Text: c}, nil
}

func NewQuantityValue(magnitude, unitCode string) (Value, error) {
	c, err := CanonicalDecimalString(magnitude)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValueQuantity, Text: c, UnitCode: unitCode}, nil
}

func NotApplicableValue() Value { return Value{Kind: ValueNotApplicable} }
