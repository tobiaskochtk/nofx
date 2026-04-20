package store

import "testing"

func TestPaperWalletSeededOnPaperExchangeCreate(t *testing.T) {
	root := newPositionHistoryTestStore(t, "paper-wallet-seed.db")

	exchangeID, err := root.Exchange().Create(
		"user-paper",
		"bybit",
		"Paper Wallet",
		true,
		"",
		"",
		"",
		false,
		ExecutionEnvironmentPaper,
		1250,
		"usdt",
		DefaultBybitPaperFeeBps,
		DefaultPaperSlippageBps,
		true,
		true,
		"",
		true,
		"",
		"",
		"",
		"",
		"",
		"",
		0,
	)
	if err != nil {
		t.Fatalf("Exchange().Create() error = %v", err)
	}

	wallet, err := root.PaperWallet().GetByExchangeID("user-paper", exchangeID)
	if err != nil {
		t.Fatalf("PaperWallet().GetByExchangeID() error = %v", err)
	}
	if wallet.SourceVenue != "bybit" {
		t.Fatalf("wallet.SourceVenue = %q, want bybit", wallet.SourceVenue)
	}
	if wallet.Asset != "USDT" {
		t.Fatalf("wallet.Asset = %q, want USDT", wallet.Asset)
	}
	if wallet.StartingBalance != 1250 {
		t.Fatalf("wallet.StartingBalance = %.2f, want 1250", wallet.StartingBalance)
	}
	if wallet.AvailableBalance != 1250 {
		t.Fatalf("wallet.AvailableBalance = %.2f, want 1250", wallet.AvailableBalance)
	}
	if wallet.Equity != 1250 {
		t.Fatalf("wallet.Equity = %.2f, want 1250", wallet.Equity)
	}
	if wallet.SessionID == "" {
		t.Fatal("wallet.SessionID should be populated")
	}

	var ledger []PaperWalletLedgerEntry
	if err := root.GormDB().Where("exchange_id = ?", exchangeID).Find(&ledger).Error; err != nil {
		t.Fatalf("Find(ledger) error = %v", err)
	}
	if len(ledger) != 1 {
		t.Fatalf("len(ledger) = %d, want 1", len(ledger))
	}
	if ledger[0].EventType != PaperWalletLedgerEventSeed {
		t.Fatalf("ledger[0].EventType = %q, want %q", ledger[0].EventType, PaperWalletLedgerEventSeed)
	}
}

func TestPaperWalletSeedSyncsWhenStillPristine(t *testing.T) {
	root := newPositionHistoryTestStore(t, "paper-wallet-config-sync.db")

	exchangeID, err := root.Exchange().Create(
		"user-paper",
		"bybit",
		"Paper Wallet",
		true,
		"",
		"",
		"",
		false,
		ExecutionEnvironmentPaper,
		1000,
		"USDT",
		DefaultBybitPaperFeeBps,
		DefaultPaperSlippageBps,
		true,
		true,
		"",
		true,
		"",
		"",
		"",
		"",
		"",
		"",
		0,
	)
	if err != nil {
		t.Fatalf("Exchange().Create() error = %v", err)
	}

	if err := root.Exchange().Update(
		"user-paper",
		exchangeID,
		true,
		"",
		"",
		"",
		false,
		ExecutionEnvironmentPaper,
		1600,
		"USDT",
		DefaultBybitPaperFeeBps,
		DefaultPaperSlippageBps,
		true,
		true,
		false,
		"",
		true,
		"",
		"",
		"",
		"",
		"",
		"",
		0,
	); err != nil {
		t.Fatalf("Exchange().Update() error = %v", err)
	}

	wallet, err := root.PaperWallet().GetByExchangeID("user-paper", exchangeID)
	if err != nil {
		t.Fatalf("PaperWallet().GetByExchangeID() error = %v", err)
	}
	if wallet.StartingBalance != 1600 {
		t.Fatalf("wallet.StartingBalance = %.2f, want 1600", wallet.StartingBalance)
	}
	if wallet.AvailableBalance != 1600 {
		t.Fatalf("wallet.AvailableBalance = %.2f, want 1600", wallet.AvailableBalance)
	}
	if wallet.Equity != 1600 {
		t.Fatalf("wallet.Equity = %.2f, want 1600", wallet.Equity)
	}

	var ledger []PaperWalletLedgerEntry
	if err := root.GormDB().Where("exchange_id = ?", exchangeID).Order("created_at ASC").Find(&ledger).Error; err != nil {
		t.Fatalf("Find(ledger) error = %v", err)
	}
	if len(ledger) != 2 {
		t.Fatalf("len(ledger) = %d, want 2", len(ledger))
	}
	if ledger[1].EventType != PaperWalletLedgerEventConfigSync {
		t.Fatalf("ledger[1].EventType = %q, want %q", ledger[1].EventType, PaperWalletLedgerEventConfigSync)
	}
}

func TestPaperWalletSeedDoesNotOverwriteActiveWallet(t *testing.T) {
	root := newPositionHistoryTestStore(t, "paper-wallet-no-overwrite.db")

	exchangeID, err := root.Exchange().Create(
		"user-paper",
		"bybit",
		"Paper Wallet",
		true,
		"",
		"",
		"",
		false,
		ExecutionEnvironmentPaper,
		1000,
		"USDT",
		DefaultBybitPaperFeeBps,
		DefaultPaperSlippageBps,
		true,
		true,
		"",
		true,
		"",
		"",
		"",
		"",
		"",
		"",
		0,
	)
	if err != nil {
		t.Fatalf("Exchange().Create() error = %v", err)
	}

	wallet, err := root.PaperWallet().GetByExchangeID("user-paper", exchangeID)
	if err != nil {
		t.Fatalf("PaperWallet().GetByExchangeID() error = %v", err)
	}
	wallet.AvailableBalance = 1012
	wallet.Equity = 1012
	wallet.RealizedPnL = 12
	if err := root.GormDB().Save(wallet).Error; err != nil {
		t.Fatalf("Save(wallet) error = %v", err)
	}

	if err := root.Exchange().Update(
		"user-paper",
		exchangeID,
		true,
		"",
		"",
		"",
		false,
		ExecutionEnvironmentPaper,
		2200,
		"USDT",
		DefaultBybitPaperFeeBps,
		DefaultPaperSlippageBps,
		true,
		true,
		false,
		"",
		true,
		"",
		"",
		"",
		"",
		"",
		"",
		0,
	); err != nil {
		t.Fatalf("Exchange().Update() error = %v", err)
	}

	updatedWallet, err := root.PaperWallet().GetByExchangeID("user-paper", exchangeID)
	if err != nil {
		t.Fatalf("PaperWallet().GetByExchangeID() error = %v", err)
	}
	if updatedWallet.StartingBalance != 1000 {
		t.Fatalf("updatedWallet.StartingBalance = %.2f, want 1000", updatedWallet.StartingBalance)
	}
	if updatedWallet.Equity != 1012 {
		t.Fatalf("updatedWallet.Equity = %.2f, want 1012", updatedWallet.Equity)
	}

	var ledgerCount int64
	if err := root.GormDB().Model(&PaperWalletLedgerEntry{}).Where("exchange_id = ?", exchangeID).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("Count(ledger) error = %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("ledgerCount = %d, want 1", ledgerCount)
	}
}
