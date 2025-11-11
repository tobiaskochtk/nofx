package main

import (
	"log"
	"nofx/market"
)

func main() {
	log.Println("Testing derivs snapshot for LINKUSDT...")
	data, err := market.Get("LINKUSDT")
	if err != nil {
		log.Fatalf("Failed to get market data: %v", err)
	}
	
	if data.Snapshot != nil && data.Snapshot.Features.Derivs != nil {
		log.Printf("✓ Derivs features loaded!")
		log.Printf("  OI Delta 1h: %v", data.Snapshot.Features.Derivs.OIDelta1hPct)
		log.Printf("  OI Z 7d: %v", data.Snapshot.Features.Derivs.OIZ7d)
		log.Printf("  OI Price Div: %v", data.Snapshot.Features.Derivs.OIPriceDiv)
		log.Printf("  OI Price Corr 24h: %v", data.Snapshot.Features.Derivs.OIPriceCorr24h)
	} else {
		log.Printf("⚠️ No derivs features in snapshot")
	}
	
	formatted := data.Format()
	log.Printf("\n=== Formatted Market Data ===\n%s\n", formatted)
}
