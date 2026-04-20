package store

import (
	"fmt"
	"nofx/crypto"
	"nofx/logger"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExchangeStore exchange storage
type ExchangeStore struct {
	db *gorm.DB
}

const (
	ExecutionEnvironmentLive    = "live"
	ExecutionEnvironmentTestnet = "testnet"
	ExecutionEnvironmentPaper   = "paper"

	DefaultPaperAsset       = "USDT"
	DefaultBybitPaperFeeBps = 5.5
	DefaultPaperSlippageBps = 0.0
)

// Exchange exchange configuration
type Exchange struct {
	ID                      string                 `gorm:"primaryKey" json:"id"`
	ExchangeType            string                 `gorm:"column:exchange_type;not null;default:''" json:"exchange_type"`
	AccountName             string                 `gorm:"column:account_name;not null;default:''" json:"account_name"`
	UserID                  string                 `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	Name                    string                 `gorm:"not null" json:"name"`
	Type                    string                 `gorm:"not null" json:"type"` // "cex" or "dex"
	Enabled                 bool                   `gorm:"default:false" json:"enabled"`
	APIKey                  crypto.EncryptedString `gorm:"column:api_key;default:''" json:"apiKey"`
	SecretKey               crypto.EncryptedString `gorm:"column:secret_key;default:''" json:"secretKey"`
	Passphrase              crypto.EncryptedString `gorm:"column:passphrase;default:''" json:"passphrase"`
	Testnet                 bool                   `gorm:"default:false" json:"testnet"`
	ExecutionEnvironment    string                 `gorm:"column:execution_environment;not null;default:live" json:"execution_environment"`
	PaperInitialBalance     float64                `gorm:"column:paper_initial_balance;default:0" json:"paper_initial_balance"`
	PaperAsset              string                 `gorm:"column:paper_asset;default:USDT" json:"paper_asset"`
	PaperFeeBps             float64                `gorm:"column:paper_fee_bps;default:5.5" json:"paper_fee_bps"`
	PaperSlippageBps        float64                `gorm:"column:paper_slippage_bps;default:0" json:"paper_slippage_bps"`
	PaperFundingEnabled     bool                   `gorm:"column:paper_funding_enabled;default:true" json:"paper_funding_enabled"`
	PaperLiquidationEnabled bool                   `gorm:"column:paper_liquidation_enabled;default:true" json:"paper_liquidation_enabled"`
	HyperliquidWalletAddr   string                 `gorm:"column:hyperliquid_wallet_addr;default:''" json:"hyperliquidWalletAddr"`
	HyperliquidUnifiedAcct  bool                   `gorm:"column:hyperliquid_unified_account;default:true" json:"hyperliquidUnifiedAccount"` // Unified Account mode (Spot as collateral)
	AsterUser               string                 `gorm:"column:aster_user;default:''" json:"asterUser"`
	AsterSigner             string                 `gorm:"column:aster_signer;default:''" json:"asterSigner"`
	AsterPrivateKey         crypto.EncryptedString `gorm:"column:aster_private_key;default:''" json:"asterPrivateKey"`
	LighterWalletAddr       string                 `gorm:"column:lighter_wallet_addr;default:''" json:"lighterWalletAddr"`
	LighterPrivateKey       crypto.EncryptedString `gorm:"column:lighter_private_key;default:''" json:"lighterPrivateKey"`
	LighterAPIKeyPrivateKey crypto.EncryptedString `gorm:"column:lighter_api_key_private_key;default:''" json:"lighterAPIKeyPrivateKey"`
	LighterAPIKeyIndex      int                    `gorm:"column:lighter_api_key_index;default:0" json:"lighterAPIKeyIndex"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

func (Exchange) TableName() string { return "exchanges" }

// NewExchangeStore creates a new ExchangeStore
func NewExchangeStore(db *gorm.DB) *ExchangeStore {
	return &ExchangeStore{db: db}
}

func NormalizeExecutionEnvironment(value string, testnet bool) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ExecutionEnvironmentPaper:
		return ExecutionEnvironmentPaper
	case ExecutionEnvironmentTestnet:
		return ExecutionEnvironmentTestnet
	case ExecutionEnvironmentLive:
		return ExecutionEnvironmentLive
	default:
		if testnet {
			return ExecutionEnvironmentTestnet
		}
		return ExecutionEnvironmentLive
	}
}

func normalizePaperAsset(value string) string {
	asset := strings.ToUpper(strings.TrimSpace(value))
	if asset == "" {
		return DefaultPaperAsset
	}
	return asset
}

func (e *Exchange) ResolvedExecutionEnvironment() string {
	if e == nil {
		return ExecutionEnvironmentLive
	}
	return NormalizeExecutionEnvironment(e.ExecutionEnvironment, e.Testnet)
}

func (e *Exchange) IsPaper() bool {
	return e != nil && e.ResolvedExecutionEnvironment() == ExecutionEnvironmentPaper
}

func (e *Exchange) IsTestnetEnvironment() bool {
	return e != nil && e.ResolvedExecutionEnvironment() == ExecutionEnvironmentTestnet
}

func (s *ExchangeStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var tableExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'exchanges'`).Scan(&tableExists)
		if tableExists > 0 {
			// Still run data migrations
			s.migrateToMultiAccount()
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS execution_environment TEXT NOT NULL DEFAULT 'live'`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_initial_balance DOUBLE PRECISION NOT NULL DEFAULT 0`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_asset TEXT NOT NULL DEFAULT 'USDT'`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_fee_bps DOUBLE PRECISION NOT NULL DEFAULT 5.5`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_slippage_bps DOUBLE PRECISION NOT NULL DEFAULT 0`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_funding_enabled BOOLEAN NOT NULL DEFAULT true`)
			s.db.Exec(`ALTER TABLE exchanges ADD COLUMN IF NOT EXISTS paper_liquidation_enabled BOOLEAN NOT NULL DEFAULT true`)
			s.db.Exec(`UPDATE exchanges
				SET execution_environment = CASE
					WHEN COALESCE(execution_environment, '') != '' THEN execution_environment
					WHEN COALESCE(testnet, false) THEN 'testnet'
					ELSE 'live'
				END`)
			s.db.Exec(`UPDATE exchanges SET paper_asset = 'USDT' WHERE COALESCE(paper_asset, '') = ''`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN execution_environment SET DEFAULT 'live'`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN paper_asset SET DEFAULT 'USDT'`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN paper_fee_bps SET DEFAULT 5.5`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN paper_slippage_bps SET DEFAULT 0`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN paper_funding_enabled SET DEFAULT true`)
			s.db.Exec(`ALTER TABLE exchanges ALTER COLUMN paper_liquidation_enabled SET DEFAULT true`)
			s.db.Model(&Exchange{}).Where("account_name = '' OR account_name IS NULL").Update("account_name", "Default")
			return nil
		}
	}

	if err := s.db.AutoMigrate(&Exchange{}); err != nil {
		return err
	}

	// Run migration to multi-account if needed
	if err := s.migrateToMultiAccount(); err != nil {
		logger.Warnf("Multi-account migration warning: %v", err)
	}

	// Fix empty account_name for existing records
	s.db.Model(&Exchange{}).Where("account_name = '' OR account_name IS NULL").Update("account_name", "Default")

	return nil
}

// migrateToMultiAccount migrates old schema (id=exchange_type) to new schema (id=UUID)
func (s *ExchangeStore) migrateToMultiAccount() error {
	// Check if migration is needed by looking for old-style IDs (non-UUID)
	var count int64
	err := s.db.Model(&Exchange{}).
		Where("exchange_type = '' AND id IN ?", []string{"binance", "bybit", "okx", "bitget", "hyperliquid", "aster", "lighter"}).
		Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		return nil
	}

	logger.Infof("🔄 Migrating %d exchange records to multi-account schema...", count)

	// Get all old records
	var records []Exchange
	err = s.db.Where("exchange_type = '' AND id IN ?", []string{"binance", "bybit", "okx", "bitget", "hyperliquid", "aster", "lighter"}).
		Find(&records).Error
	if err != nil {
		return err
	}

	// Begin transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, r := range records {
			newID := uuid.New().String()
			oldID := r.ID // This is the exchange type (e.g., "binance")

			// Update traders table to use new UUID
			if err := tx.Exec("UPDATE traders SET exchange_id = ? WHERE exchange_id = ? AND user_id = ?",
				newID, oldID, r.UserID).Error; err != nil {
				logger.Errorf("Failed to update traders for exchange %s: %v", oldID, err)
				return err
			}

			// Update the exchange record
			if err := tx.Model(&Exchange{}).
				Where("id = ? AND user_id = ?", oldID, r.UserID).
				Updates(map[string]interface{}{
					"id":            newID,
					"exchange_type": oldID,
					"account_name":  "Default",
				}).Error; err != nil {
				logger.Errorf("Failed to migrate exchange %s: %v", oldID, err)
				return err
			}

			logger.Infof("✅ Migrated exchange %s -> UUID %s for user %s", oldID, newID, r.UserID)
		}
		return nil
	})
}

func (s *ExchangeStore) initDefaultData() error {
	// No longer pre-populate exchanges - create on demand when user configures
	return nil
}

// List gets user's exchange list
func (s *ExchangeStore) List(userID string) ([]*Exchange, error) {
	var exchanges []*Exchange
	err := s.db.Where("user_id = ?", userID).Order("exchange_type, account_name").Find(&exchanges).Error
	if err != nil {
		return nil, err
	}
	return exchanges, nil
}

// GetByID gets a specific exchange by UUID
func (s *ExchangeStore) GetByID(userID, id string) (*Exchange, error) {
	var exchange Exchange
	err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&exchange).Error
	if err != nil {
		return nil, err
	}
	return &exchange, nil
}

// getExchangeNameAndType returns the display name and type for an exchange type
func getExchangeNameAndType(exchangeType string) (name string, typ string) {
	switch exchangeType {
	case "binance":
		return "Binance Futures", "cex"
	case "bybit":
		return "Bybit Futures", "cex"
	case "okx":
		return "OKX Futures", "cex"
	case "bitget":
		return "Bitget Futures", "cex"
	case "hyperliquid":
		return "Hyperliquid", "dex"
	case "aster":
		return "Aster DEX", "dex"
	case "lighter":
		return "LIGHTER DEX", "dex"
	case "indodax":
		return "Indodax", "cex"
	default:
		return exchangeType + " Exchange", "cex"
	}
}

// Create creates a new exchange account with UUID
func (s *ExchangeStore) Create(userID, exchangeType, accountName string, enabled bool,
	apiKey, secretKey, passphrase string, testnet bool, executionEnvironment string,
	paperInitialBalance float64, paperAsset string, paperFeeBps, paperSlippageBps float64,
	paperFundingEnabled, paperLiquidationEnabled bool,
	hyperliquidWalletAddr string, hyperliquidUnifiedAcct bool,
	asterUser, asterSigner, asterPrivateKey,
	lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string, lighterApiKeyIndex int) (string, error) {

	id := uuid.New().String()
	name, typ := getExchangeNameAndType(exchangeType)

	if accountName == "" {
		accountName = "Default"
	}

	logger.Debugf("🔧 ExchangeStore.Create: userID=%s, exchangeType=%s, accountName=%s, id=%s",
		userID, exchangeType, accountName, id)

	normalizedEnvironment := NormalizeExecutionEnvironment(executionEnvironment, testnet)
	testnet = normalizedEnvironment == ExecutionEnvironmentTestnet
	paperAsset = normalizePaperAsset(paperAsset)
	if normalizedEnvironment != ExecutionEnvironmentPaper {
		paperInitialBalance = 0
		paperAsset = DefaultPaperAsset
		paperFeeBps = DefaultBybitPaperFeeBps
		paperSlippageBps = DefaultPaperSlippageBps
		paperFundingEnabled = true
		paperLiquidationEnabled = true
	}

	exchange := &Exchange{
		ID:                      id,
		ExchangeType:            exchangeType,
		AccountName:             accountName,
		UserID:                  userID,
		Name:                    name,
		Type:                    typ,
		Enabled:                 enabled,
		APIKey:                  crypto.EncryptedString(apiKey),
		SecretKey:               crypto.EncryptedString(secretKey),
		Passphrase:              crypto.EncryptedString(passphrase),
		Testnet:                 testnet,
		ExecutionEnvironment:    normalizedEnvironment,
		PaperInitialBalance:     paperInitialBalance,
		PaperAsset:              paperAsset,
		PaperFeeBps:             paperFeeBps,
		PaperSlippageBps:        paperSlippageBps,
		PaperFundingEnabled:     paperFundingEnabled,
		PaperLiquidationEnabled: paperLiquidationEnabled,
		HyperliquidWalletAddr:   hyperliquidWalletAddr,
		HyperliquidUnifiedAcct:  hyperliquidUnifiedAcct,
		AsterUser:               asterUser,
		AsterSigner:             asterSigner,
		AsterPrivateKey:         crypto.EncryptedString(asterPrivateKey),
		LighterWalletAddr:       lighterWalletAddr,
		LighterPrivateKey:       crypto.EncryptedString(lighterPrivateKey),
		LighterAPIKeyPrivateKey: crypto.EncryptedString(lighterApiKeyPrivateKey),
		LighterAPIKeyIndex:      lighterApiKeyIndex,
	}

	if err := s.db.Create(exchange).Error; err != nil {
		return "", err
	}
	if exchange.IsPaper() {
		if _, err := NewPaperWalletStore(s.db).EnsureForExchange(exchange); err != nil {
			return "", err
		}
	}
	return id, nil
}

// Update updates exchange configuration by UUID
func (s *ExchangeStore) Update(userID, id string, enabled bool, apiKey, secretKey, passphrase string, testnet bool, executionEnvironment string,
	paperInitialBalance float64, paperAsset string, paperFeeBps, paperSlippageBps float64,
	paperFundingEnabled, paperLiquidationEnabled bool, clearCredentials bool,
	hyperliquidWalletAddr string, hyperliquidUnifiedAcct bool,
	asterUser, asterSigner, asterPrivateKey, lighterWalletAddr, lighterPrivateKey, lighterApiKeyPrivateKey string, lighterApiKeyIndex int) error {

	logger.Debugf("🔧 ExchangeStore.Update: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	normalizedEnvironment := NormalizeExecutionEnvironment(executionEnvironment, testnet)
	testnet = normalizedEnvironment == ExecutionEnvironmentTestnet
	paperAsset = normalizePaperAsset(paperAsset)
	if normalizedEnvironment != ExecutionEnvironmentPaper {
		paperInitialBalance = 0
		paperAsset = DefaultPaperAsset
		paperFeeBps = DefaultBybitPaperFeeBps
		paperSlippageBps = DefaultPaperSlippageBps
		paperFundingEnabled = true
		paperLiquidationEnabled = true
	}

	updates := map[string]interface{}{
		"enabled":                     enabled,
		"testnet":                     testnet,
		"execution_environment":       normalizedEnvironment,
		"paper_initial_balance":       paperInitialBalance,
		"paper_asset":                 paperAsset,
		"paper_fee_bps":               paperFeeBps,
		"paper_slippage_bps":          paperSlippageBps,
		"paper_funding_enabled":       paperFundingEnabled,
		"paper_liquidation_enabled":   paperLiquidationEnabled,
		"hyperliquid_wallet_addr":     hyperliquidWalletAddr,
		"hyperliquid_unified_account": hyperliquidUnifiedAcct,
		"aster_user":                  asterUser,
		"aster_signer":                asterSigner,
		"lighter_wallet_addr":         lighterWalletAddr,
		"lighter_api_key_index":       lighterApiKeyIndex,
		"updated_at":                  time.Now().UTC(),
	}

	if clearCredentials {
		updates["api_key"] = crypto.EncryptedString("")
		updates["secret_key"] = crypto.EncryptedString("")
		updates["passphrase"] = crypto.EncryptedString("")
		updates["aster_private_key"] = crypto.EncryptedString("")
		updates["lighter_private_key"] = crypto.EncryptedString("")
		updates["lighter_api_key_private_key"] = crypto.EncryptedString("")
	}

	// Only update encrypted fields if not empty
	if !clearCredentials && apiKey != "" {
		updates["api_key"] = crypto.EncryptedString(apiKey)
	}
	if !clearCredentials && secretKey != "" {
		updates["secret_key"] = crypto.EncryptedString(secretKey)
	}
	if !clearCredentials && passphrase != "" {
		updates["passphrase"] = crypto.EncryptedString(passphrase)
	}
	if !clearCredentials && asterPrivateKey != "" {
		updates["aster_private_key"] = crypto.EncryptedString(asterPrivateKey)
	}
	if !clearCredentials && lighterPrivateKey != "" {
		updates["lighter_private_key"] = crypto.EncryptedString(lighterPrivateKey)
	}
	if !clearCredentials && lighterApiKeyPrivateKey != "" {
		updates["lighter_api_key_private_key"] = crypto.EncryptedString(lighterApiKeyPrivateKey)
	}

	result := s.db.Model(&Exchange{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	if normalizedEnvironment == ExecutionEnvironmentPaper {
		exchange, err := s.GetByID(userID, id)
		if err != nil {
			return err
		}
		if _, err := NewPaperWalletStore(s.db).EnsureForExchange(exchange); err != nil {
			return err
		}
	}
	return nil
}

// UpdateAccountName updates the account name for an exchange
func (s *ExchangeStore) UpdateAccountName(userID, id, accountName string) error {
	result := s.db.Model(&Exchange{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{
			"account_name": accountName,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	return nil
}

// Delete deletes an exchange account
func (s *ExchangeStore) Delete(userID, id string) error {
	result := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Exchange{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("exchange not found: id=%s, userID=%s", id, userID)
	}
	logger.Infof("🗑️ Deleted exchange: id=%s, userID=%s", id, userID)
	return nil
}

// CreateLegacy creates exchange configuration (legacy API for backward compatibility)
// This method is deprecated, use Create instead
func (s *ExchangeStore) CreateLegacy(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool,
	hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {

	// Check if this is an old-style ID (exchange type as ID)
	if id == "binance" || id == "bybit" || id == "okx" || id == "bitget" || id == "hyperliquid" || id == "aster" || id == "lighter" {
		_, err := s.Create(userID, id, "Default", enabled, apiKey, secretKey, "", testnet, "",
			0, DefaultPaperAsset, DefaultBybitPaperFeeBps, DefaultPaperSlippageBps, true, true,
			hyperliquidWalletAddr, true, // Default to Unified Account mode
			asterUser, asterSigner, asterPrivateKey, "", "", "", 0)
		return err
	}

	// Otherwise assume it's already a UUID
	exchange := &Exchange{
		ID:                    id,
		UserID:                userID,
		Name:                  name,
		Type:                  typ,
		Enabled:               enabled,
		APIKey:                crypto.EncryptedString(apiKey),
		SecretKey:             crypto.EncryptedString(secretKey),
		Testnet:               testnet,
		ExecutionEnvironment:  NormalizeExecutionEnvironment("", testnet),
		PaperAsset:            DefaultPaperAsset,
		PaperFeeBps:           DefaultBybitPaperFeeBps,
		HyperliquidWalletAddr: hyperliquidWalletAddr,
		AsterUser:             asterUser,
		AsterSigner:           asterSigner,
		AsterPrivateKey:       crypto.EncryptedString(asterPrivateKey),
	}
	return s.db.Where("id = ?", id).FirstOrCreate(exchange).Error
}
