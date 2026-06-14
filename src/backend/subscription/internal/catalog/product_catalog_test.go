package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/subscription/internal/testfixtures"
)

// TestProductCatalog_YearlyPremiumDiscount20Percent documents −20% yearly SKU pricing.
func TestProductCatalog_YearlyPremiumDiscount20Percent(t *testing.T) {
	t.Parallel()

	cat := Default()
	monthly, ok := cat.PriceCents("premium", "monthly")
	require.True(t, ok, "premium monthly SKU must exist")
	require.Equal(t, testfixtures.PremiumMonthlyPriceCents, monthly)

	yearly, ok := cat.PriceCents("premium", "yearly")
	require.True(t, ok, "premium yearly SKU must exist")

	wantYearly := int(float64(monthly*12) * (1 - float64(testfixtures.YearlyDiscountPercent)/100))
	require.Equal(t, wantYearly, yearly)
}

// TestProductCatalog_YearlySpaceProDiscount20Percent documents Space Pro yearly discount.
func TestProductCatalog_YearlySpaceProDiscount20Percent(t *testing.T) {
	t.Parallel()

	cat := Default()
	monthly, ok := cat.PriceCents("space_pro", "monthly")
	require.True(t, ok, "space_pro monthly SKU must exist")
	require.Equal(t, testfixtures.PremiumMonthlyPriceCents, monthly)

	yearly, ok := cat.PriceCents("space_pro", "yearly")
	require.True(t, ok, "space_pro yearly SKU must exist")

	wantYearly := int(float64(monthly*12) * (1 - float64(testfixtures.YearlyDiscountPercent)/100))
	require.Equal(t, wantYearly, yearly)
}
