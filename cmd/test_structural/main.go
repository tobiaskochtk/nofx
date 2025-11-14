package main

import (
	"log"
	"nofx/market"
)

func main() {
	log.Println("🧪 Testing Data Availability for ALL Trading Symbols")
	log.Println("====================================================")

	api := market.NewAPIClient()

	// Test symbols from your trading list
	symbols := []string{
		"BTCUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT",
		"LINKUSDT", "TRXUSDT", "LTCUSDT",
		"UNIUSDT", "ARBUSDT", "ETHUSDT",
		"AAVEUSDT", "SOLUSDT",
	}

	successCount := 0
	failCount := 0
	insufficientCount := 0

	for _, symbol := range symbols {
		klines, err := api.GetKlines(symbol, "3m", 600)
		if err != nil {
			log.Printf("❌ %s: API ERROR - %v", symbol, err)
			failCount++
			continue
		}

		if len(klines) < 240 {
			log.Printf("⚠️  %s: Only %d bars (need 240+)", symbol, len(klines))
			insufficientCount++
		} else {
			log.Printf("✅ %s: %d bars available", symbol, len(klines))
			successCount++
		}
	}

	log.Println("")
	log.Println("📊 Summary:")
	log.Printf("   ✅ Ready: %d symbols", successCount)
	log.Printf("   ⚠️  Insufficient: %d symbols", insufficientCount)
	log.Printf("   ❌ Failed: %d symbols", failCount)
	log.Printf("   📈 Total tested: %d symbols", len(symbols))

	if successCount == len(symbols) {
		log.Println("")
		log.Println("🎉 ALL SYMBOLS READY - Features will work for all assets!")
	} else if failCount > 0 {
		log.Println("")
		log.Println("⚠️  Some symbols have API issues - check errors above")
	}
}
