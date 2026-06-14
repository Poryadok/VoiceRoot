package catalog

import "voice/backend/subscription/internal/testfixtures"

// ProductCatalog lists billable SKUs (Premium, Space Pro).
type ProductCatalog struct {
	skus map[string]int
}

// Default returns the product catalog for checkout (Phase 12).
func Default() *ProductCatalog {
	monthly := testfixtures.PremiumMonthlyPriceCents
	yearly := int(float64(monthly*12) * (1 - float64(testfixtures.YearlyDiscountPercent)/100))
	return &ProductCatalog{skus: map[string]int{
		"premium:monthly":   monthly,
		"premium:yearly":    yearly,
		"space_pro:monthly": monthly,
		"space_pro:yearly":  yearly,
	}}
}

// PriceCents returns SKU price in cents for plan (premium|space_pro) and period (monthly|yearly).
func (c *ProductCatalog) PriceCents(plan, period string) (int, bool) {
	if c == nil || c.skus == nil {
		return 0, false
	}
	v, ok := c.skus[plan+":"+period]
	return v, ok
}
