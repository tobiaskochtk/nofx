package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nofx/logger"

	_ "modernc.org/sqlite"
)

type dealRecord struct {
	ID         int64
	UserID     string
	TraderID   string
	Symbol     string
	Side       string
	Leverage   int
	Quantity   float64
	OpenPrice  float64
	OpenTime   time.Time
	CloseTime  sql.NullTime
	ClosePrice sql.NullFloat64
}

var (
	dbPath   = flag.String("db", "config.db", "Path to SQLite database")
	logsDir  = flag.String("logs", "decision_logs", "Decision logs root directory")
	dryRun   = flag.Bool("dry", false, "Dry run (do not update database)")
	maxDeals = flag.Int("limit", 0, "Limit number of deals to process (0 = all)")
)

func main() {
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deals, err := loadTargetDeals(db, *maxDeals)
	if err != nil {
		log.Fatalf("load deals: %v", err)
	}
	if len(deals) == 0 {
		log.Println("✅ no deals to fix")
		return
	}

	logCache := make(map[string][]string)
	fixed := 0
	for _, deal := range deals {
		price, closeTime, orderID := fetchCloseInfo(db, deal)
		if price <= 0 {
			p, t, oid := findPriceFromLogs(*logsDir, logCache, deal)
			price, closeTime, orderID = p, t, oid
		}
		if price <= 0 {
			log.Printf("⚠️ deal %d (%s %s) missing close price, skip", deal.ID, deal.Symbol, deal.Side)
			continue
		}
		if orderID == "" {
			orderID = "historical_fix"
		}
		if closeTime.IsZero() {
			closeTime = time.Now()
		}

		pnl, pnlPct := computeDealPnL(deal, price)
		duration := closeTime.Sub(deal.OpenTime).Seconds()
		log.Printf("deal %d %s %s close=%.6f pnl=%.4f (%.4f%%)", deal.ID, deal.Symbol, deal.Side, price, pnl, pnlPct)

		if *dryRun {
			continue
		}
		if err := updateDeal(db, deal, price, closeTime, pnl, pnlPct, int64(duration), orderID); err != nil {
			log.Printf("❌ update deal %d failed: %v", deal.ID, err)
			continue
		}
		fixed++
	}
	log.Printf("✅ finished. fixed=%d/%d", fixed, len(deals))
}

func loadTargetDeals(db *sql.DB, limit int) ([]dealRecord, error) {
	query := `
SELECT id, user_id, trader_id, symbol, side, leverage, quantity, open_price, open_time,
       COALESCE(close_time, '') as close_time,
       COALESCE(close_price, 0) as close_price
FROM deals
WHERE status = 'closed'
  AND (
    close_price IS NULL OR close_price = 0 OR
    realized_pnl IS NULL OR ABS(realized_pnl) < 1e-9 OR
    realized_pnl_pct IS NULL OR ABS(realized_pnl_pct) < 1e-9
  )
ORDER BY open_time ASC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []dealRecord
	for rows.Next() {
		var d dealRecord
		var openTimeStr, closeTimeStr string
		if err := rows.Scan(&d.ID, &d.UserID, &d.TraderID, &d.Symbol, &d.Side, &d.Leverage, &d.Quantity, &d.OpenPrice, &openTimeStr, &closeTimeStr, &d.ClosePrice); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02 15:04:05", openTimeStr)
		if err != nil {
			return nil, fmt.Errorf("parse open_time: %w", err)
		}
		d.OpenTime = t
		if closeTimeStr != "" {
			if ct, err := time.Parse("2006-01-02 15:04:05", closeTimeStr); err == nil {
				d.CloseTime = sql.NullTime{Time: ct, Valid: true}
			}
		}
		deals = append(deals, d)
	}
	return deals, nil
}

func fetchCloseInfo(db *sql.DB, deal dealRecord) (float64, time.Time, string) {
	// deal_events first
	var price sql.NullFloat64
	var ts sql.NullString
	var orderID sql.NullString
	err := db.QueryRow(`SELECT price, created_at, order_id FROM deal_events WHERE deal_id = ? AND type = 'close' ORDER BY id DESC LIMIT 1`, deal.ID).
		Scan(&price, &ts, &orderID)
	if err == nil && price.Valid && price.Float64 > 0 {
		t := time.Now()
		if ts.Valid {
			if parsed, err := time.Parse("2006-01-02 15:04:05", ts.String); err == nil {
				t = parsed
			}
		}
		return price.Float64, t, orderID.String
	}
	return 0, time.Time{}, ""
}

func findPriceFromLogs(root string, cache map[string][]string, deal dealRecord) (float64, time.Time, string) {
	files, err := getLogFiles(root, deal.TraderID, cache)
	if err != nil {
		log.Printf("⚠️ load logs for trader %s: %v", deal.TraderID, err)
		return 0, time.Time{}, ""
	}
	for _, file := range files {
		rec, err := readDecisionRecord(file)
		if err != nil {
			continue
		}
		for _, action := range rec.Decisions {
			if !isCloseAction(action.Action) {
				continue
			}
			if strings.EqualFold(action.Symbol, deal.Symbol) && strings.Contains(strings.ToLower(action.Action), deal.Side) {
				if action.Timestamp.After(deal.OpenTime) && action.Price > 0 {
					orderID := fmt.Sprintf("%d", action.OrderID)
					return action.Price, action.Timestamp, orderID
				}
			}
		}
	}
	return 0, time.Time{}, ""
}

func getLogFiles(root, traderID string, cache map[string][]string) ([]string, error) {
	if files, ok := cache[traderID]; ok {
		return files, nil
	}
	dir := filepath.Join(root, traderID)
	pattern := filepath.Join(dir, "decision_*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	cache[traderID] = matches
	return matches, nil
}

func readDecisionRecord(path string) (*logger.DecisionRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec logger.DecisionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func isCloseAction(action string) bool {
	return action == "close_long" || action == "close_short" || action == "auto_close_long" || action == "auto_close_short"
}

func computeDealPnL(deal dealRecord, closePrice float64) (float64, float64) {
	side := strings.ToLower(deal.Side)
	open := deal.OpenPrice
	slippageRate := getEnvFloat("NOFX_PNL_SLIPPAGE_RATE", 0.0005)
	feeRate := getEnvFloat("NOFX_PNL_TAKER_FEE_RATE", 0.0004)
	effOpen := open
	effClose := closePrice
	if side == "long" {
		effOpen = open * (1 + slippageRate)
		effClose = closePrice * (1 - slippageRate)
	} else {
		effOpen = open * (1 - slippageRate)
		effClose = closePrice * (1 + slippageRate)
	}
	priceChangePct := 0.0
	if effOpen > 0 {
		if side == "long" {
			priceChangePct = (effClose - effOpen) / effOpen
		} else {
			priceChangePct = (effOpen - effClose) / effOpen
		}
	}
	positionValue := deal.Quantity * effOpen
	grossPnL := positionValue * priceChangePct * float64(deal.Leverage)
	fees := (deal.Quantity*effOpen + deal.Quantity*effClose) * feeRate
	realizedPnL := grossPnL - fees
	marginUsed := 0.0
	if deal.Leverage > 0 {
		marginUsed = positionValue / float64(deal.Leverage)
	}
	realizedPnLPct := 0.0
	if marginUsed > 0 {
		realizedPnLPct = (realizedPnL / marginUsed) * 100
	}
	return realizedPnL, realizedPnLPct
}

func updateDeal(db *sql.DB, deal dealRecord, price float64, closeTime time.Time, pnl, pnlPct float64, duration int64, orderID string) error {
	_, err := db.Exec(`
UPDATE deals SET
	close_price = ?,
	close_time = COALESCE(close_time, ?),
	realized_pnl = ?,
	realized_pnl_pct = ?,
	duration_seconds = COALESCE(duration_seconds, ?),
	status = 'closed',
	updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
		price,
		closeTime.Format("2006-01-02 15:04:05"),
		pnl,
		pnlPct,
		duration,
		deal.ID,
	)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`
INSERT INTO deal_events (user_id, trader_id, deal_id, type, symbol, side, price, order_id)
VALUES (?, ?, ?, 'close', ?, ?, ?, ?)`,
		deal.UserID, deal.TraderID, deal.ID, deal.Symbol, strings.ToLower(deal.Side), price, orderID)
	return nil
}

func getEnvFloat(name string, def float64) float64 {
	if val := os.Getenv(name); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return def
}
