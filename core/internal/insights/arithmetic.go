package insights

import "math/big"

func amount(value *string) *big.Rat {
	if value == nil {
		return nil
	}
	number, valid := new(big.Rat).SetString(*value)
	if !valid {
		return nil
	}
	return number
}

// Round half away from zero, only at the presentation boundary.
func rounded(value *big.Rat) *int64 {
	if value == nil {
		return nil
	}
	result := roundInteger(value)
	// Extreme percentage ratios can exceed JSON's safe integer range.
	if result.Cmp(big.NewInt(9007199254740991)) > 0 || result.Cmp(big.NewInt(-9007199254740991)) < 0 {
		return nil
	}
	number := result.Int64()
	return &number
}

func roundInteger(value *big.Rat) *big.Int {
	numerator := new(big.Int).Abs(value.Num())
	numerator.Mul(numerator, big.NewInt(2))
	numerator.Add(numerator, value.Denom())
	denominator := new(big.Int).Mul(value.Denom(), big.NewInt(2))
	result := new(big.Int).Quo(numerator, denominator)
	if value.Sign() < 0 {
		result.Neg(result)
	}
	return result
}

func relative(current, baseline *big.Rat) *big.Rat {
	if current == nil || baseline == nil || baseline.Sign() <= 0 {
		return nil
	}
	ratio := new(big.Rat).Quo(current, baseline)
	ratio.Sub(ratio, big.NewRat(1, 1))
	return ratio.Mul(ratio, big.NewRat(10000, 1))
}

func averageRelative(series []ProductSeries, position, baseline int) *big.Rat {
	total := new(big.Rat)
	for _, product := range series {
		total = addRelative(total, product, position, baseline)
	}
	if total == nil {
		return nil
	}
	return total.Quo(total, big.NewRat(int64(len(series)), 1))
}

func addRelative(total *big.Rat, product ProductSeries, position, baseline int) *big.Rat {
	if total == nil {
		return nil
	}
	change := relative(amount(product.Points[position].Amount), amount(product.Points[baseline].Amount))
	if change == nil {
		return nil
	}
	return total.Add(total, change)
}
