package smsbower

import (
	"testing"

	"github.com/byte-v-forge/sms/internal/core"
)

func TestSelectProviderIDsByStockRespectsPriceBounds(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "cheap", Quality: "Gold", Price: core.Money{AmountDecimal: "0.054"}, AvailableCount: 20},
		{ProviderID: "next", Quality: "Gold", Price: core.Money{AmountDecimal: "0.067"}, AvailableCount: 10},
		{ProviderID: "high", Quality: "Gold", Price: core.Money{AmountDecimal: "0.120"}, AvailableCount: 10},
	}
	got := selectProviderIDsByStock(
		offers,
		5,
		core.Money{AmountDecimal: "0.055"},
		core.Money{AmountDecimal: "0.10"},
		core.Money{},
		core.Money{},
	)
	if len(got) != 1 || got[0] != "next" {
		t.Fatalf("selectProviderIDsByStock() = %v, want [next]", got)
	}
}

func TestSelectProviderIDsByStockPrefersGoldQuality(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "bronze", Quality: "Bronze", Price: core.Money{AmountDecimal: "0.050"}, AvailableCount: 20},
		{ProviderID: "silver", Quality: "Silver", Price: core.Money{AmountDecimal: "0.060"}, AvailableCount: 10},
		{ProviderID: "gold", Quality: "Gold", Price: core.Money{AmountDecimal: "0.060"}, AvailableCount: 10},
	}
	got := selectProviderIDsByStock(offers, 5)
	if len(got) != 1 || got[0] != "gold" {
		t.Fatalf("selectProviderIDsByStock() = %v, want [gold]", got)
	}
}

func TestSelectProviderIDsByStockUsesLowerSilverBeforeHigherGold(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "silver", Quality: "Silver", Price: core.Money{AmountDecimal: "0.050"}, AvailableCount: 20},
		{ProviderID: "gold", Quality: "Gold", Price: core.Money{AmountDecimal: "0.070"}, AvailableCount: 10},
	}
	got := selectProviderIDsByStock(offers, 5)
	if len(got) != 1 || got[0] != "silver" {
		t.Fatalf("selectProviderIDsByStock() = %v, want [silver]", got)
	}
}

func TestSelectProviderIDsByStockFallsBackToSilverWithoutGoldQuality(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "silver", Quality: "Silver", Price: core.Money{AmountDecimal: "0.050"}, AvailableCount: 20},
	}
	got := selectProviderIDsByStock(offers, 5)
	if len(got) != 1 || got[0] != "silver" {
		t.Fatalf("selectProviderIDsByStock() = %v, want [silver]", got)
	}
}

func TestSelectProviderIDsByStockReturnsNilWithoutGoldOrSilverQuality(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "bronze", Quality: "Bronze", Price: core.Money{AmountDecimal: "0.050"}, AvailableCount: 20},
		{ProviderID: "unknown", Price: core.Money{AmountDecimal: "0.060"}, AvailableCount: 20},
	}
	got := selectProviderIDsByStock(offers, 5)
	if got != nil {
		t.Fatalf("selectProviderIDsByStock() = %v, want nil", got)
	}
}

func TestSelectProviderIDsByStockAppliesPriceBoundsBeforeGoldCompare(t *testing.T) {
	offers := []PriceOffer{
		{ProviderID: "cheap", Quality: "Gold", Price: core.Money{AmountDecimal: "0.054"}, AvailableCount: 20},
		{ProviderID: "next", Quality: "Gold", Price: core.Money{AmountDecimal: "0.067"}, AvailableCount: 10},
		{ProviderID: "high", Quality: "Gold", Price: core.Money{AmountDecimal: "0.120"}, AvailableCount: 10},
	}
	got := selectProviderIDsByStock(
		offers,
		5,
		core.Money{AmountDecimal: "0.055"},
		core.Money{AmountDecimal: "0.10"},
		core.Money{},
		core.Money{},
	)
	if len(got) != 1 || got[0] != "next" {
		t.Fatalf("selectProviderIDsByStock() = %v, want [next]", got)
	}
}
