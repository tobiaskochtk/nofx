CREATE TABLE ai_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, custom_api_url TEXT DEFAULT '', custom_model_name TEXT DEFAULT '',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
CREATE TABLE user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, oi_symbols TEXT DEFAULT '',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		);
CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, custom_prompt TEXT DEFAULT '', override_base_prompt BOOLEAN DEFAULT 0, is_cross_margin BOOLEAN DEFAULT 1, use_default_coins BOOLEAN DEFAULT 1, custom_coins TEXT DEFAULT '', use_inside_coins BOOLEAN DEFAULT 0, system_prompt_template TEXT DEFAULT 'default', max_positions INTEGER DEFAULT 3,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		);
CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
CREATE TABLE system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
CREATE TABLE beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
CREATE TABLE deals (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id TEXT NOT NULL,
            trader_id TEXT NOT NULL,
            exchange TEXT NOT NULL,
            symbol TEXT NOT NULL,
            side TEXT NOT NULL, -- long | short
            leverage INTEGER DEFAULT 0,
            position_size_usd REAL DEFAULT 0,
            quantity REAL DEFAULT 0,
            open_price REAL DEFAULT 0,
            open_time DATETIME DEFAULT CURRENT_TIMESTAMP,
            open_order_id TEXT DEFAULT '',
            system_prompt TEXT DEFAULT '',
            user_prompt TEXT DEFAULT '',
            reasoning TEXT DEFAULT '',
            cot_trace TEXT DEFAULT '',
            decision_json TEXT DEFAULT '',
            market_context_json TEXT DEFAULT '',
            stop_loss REAL DEFAULT 0,
            take_profit REAL DEFAULT 0,
            close_price REAL,
            close_time DATETIME,
            close_order_id TEXT,
            realized_pnl REAL,
            realized_pnl_pct REAL,
            duration_seconds INTEGER,
            was_stop_loss BOOLEAN,
            status TEXT NOT NULL DEFAULT 'open', -- open | closed
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
        );
CREATE TABLE deal_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id TEXT NOT NULL,
            trader_id TEXT NOT NULL,
            deal_id INTEGER NOT NULL,
            type TEXT NOT NULL, -- open|partial_close|update_stop_loss|update_take_profit|close
            symbol TEXT NOT NULL,
            side TEXT NOT NULL,
            quantity REAL DEFAULT 0,
            percentage REAL DEFAULT 0,
            price REAL DEFAULT 0,
            order_id TEXT DEFAULT '',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (deal_id) REFERENCES deals(id) ON DELETE CASCADE
        );
CREATE TRIGGER update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_user_signal_sources_updated_at
			AFTER UPDATE ON user_signal_sources
			BEGIN
				UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END;
CREATE TRIGGER update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END;
CREATE TRIGGER update_deals_updated_at
            AFTER UPDATE ON deals
            BEGIN
                UPDATE deals SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
            END;
CREATE INDEX idx_deals_user_trader ON deals(user_id, trader_id);
CREATE INDEX idx_deals_symbol_side_status ON deals(symbol, side, status);
CREATE INDEX idx_deal_events_deal ON deal_events(deal_id);
CREATE TABLE IF NOT EXISTS "exchanges" (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
CREATE TRIGGER update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END;
