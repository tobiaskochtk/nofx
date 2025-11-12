package main

import (
	"log"

	"nofx/market"
)

func main() {
	log.Println("Initializing market monitor...")

	// Initialize the WebSocket monitor with a single symbol for testing
	monitor := market.NewWSMonitor(1)
	if err := monitor.Initialize([]string{"LINKUSDT"}); err != nil {
		log.Fatalf("Failed to initialize monitor: %v", err)
	}

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

	formatted := market.Format(data)
	log.Printf("\n=== Formatted Market Data ===\n%s\n", formatted)
}
