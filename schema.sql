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
CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			invert_signals BOOLEAN DEFAULT 0,
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
CREATE TABLE `deal_review_cases` (`id` text,`user_id` text NOT NULL,`trader_id` text NOT NULL,`position_id` integer NOT NULL,`exchange_id` text DEFAULT "",`exchange_type` text DEFAULT "",`ai_model_id` text DEFAULT "",`strategy_id` text DEFAULT "",`symbol` text NOT NULL,`side` text NOT NULL,`status` text DEFAULT "OPEN",`outcome` text DEFAULT "open",`open_event_id` text DEFAULT "",`close_event_id` text DEFAULT "",`open_cycle_number` integer DEFAULT 0,`close_cycle_number` integer DEFAULT 0,`entry_order_id` text DEFAULT "",`exit_order_id` text DEFAULT "",`entry_time_ms` integer DEFAULT 0,`exit_time_ms` integer DEFAULT 0,`entry_price` real DEFAULT 0,`exit_price` real DEFAULT 0,`entry_quantity` real DEFAULT 0,`exit_quantity` real DEFAULT 0,`leverage` integer DEFAULT 1,`open_stop_loss` real DEFAULT 0,`open_take_profit` real DEFAULT 0,`open_confidence` integer DEFAULT 0,`close_confidence` integer DEFAULT 0,`open_selection_bucket` text DEFAULT "",`open_candidate_sources_json` text DEFAULT "[]",`labels_json` text DEFAULT "[]",`analyst_note` text DEFAULT "",`realized_pnl` real DEFAULT 0,`realized_pnl_pct` real DEFAULT 0,`fee` real DEFAULT 0,`hold_duration_ms` integer DEFAULT 0,`close_reason` text DEFAULT "",`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_deal_review_cases_outcome` ON `deal_review_cases`(`outcome`);
CREATE INDEX `idx_deal_review_cases_status` ON `deal_review_cases`(`status`);
CREATE INDEX `idx_deal_review_cases_side` ON `deal_review_cases`(`side`);
CREATE INDEX `idx_deal_review_cases_symbol` ON `deal_review_cases`(`symbol`);
CREATE UNIQUE INDEX `idx_deal_review_cases_position` ON `deal_review_cases`(`position_id`);
CREATE INDEX `idx_deal_review_cases_trader_time` ON `deal_review_cases`(`trader_id`);
CREATE INDEX `idx_deal_review_cases_user_time` ON `deal_review_cases`(`user_id`);
CREATE TABLE `deal_review_events` (`id` text,`user_id` text NOT NULL,`trader_id` text NOT NULL,`deal_id` text DEFAULT "",`position_id` integer DEFAULT 0,`stage` text NOT NULL,`source` text DEFAULT "ai_decision",`status` text DEFAULT "pending",`decision_cycle_number` integer DEFAULT 0,`decision_timestamp` datetime,`exchange_id` text DEFAULT "",`exchange_order_id` text DEFAULT "",`symbol` text NOT NULL,`side` text NOT NULL,`action` text DEFAULT "",`quantity` real DEFAULT 0,`position_size_usd` real DEFAULT 0,`price` real DEFAULT 0,`leverage` integer DEFAULT 0,`stop_loss` real DEFAULT 0,`take_profit` real DEFAULT 0,`confidence` integer DEFAULT 0,`reasoning` text DEFAULT "",`selection_bucket` text DEFAULT "",`candidate_sources_json` text DEFAULT "[]",`snapshot_json` text DEFAULT "{}",`outcome_pnl` real DEFAULT 0,`outcome_pnl_pct` real DEFAULT 0,`close_reason` text DEFAULT "",`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_deal_review_events_symbol` ON `deal_review_events`(`symbol`);
CREATE INDEX `idx_deal_review_events_order` ON `deal_review_events`(`exchange_order_id`);
CREATE INDEX `idx_deal_review_events_status` ON `deal_review_events`(`status`);
CREATE INDEX `idx_deal_review_events_stage` ON `deal_review_events`(`stage`);
CREATE INDEX `idx_deal_review_events_position` ON `deal_review_events`(`position_id`);
CREATE INDEX `idx_deal_review_events_deal` ON `deal_review_events`(`deal_id`);
CREATE INDEX `idx_deal_review_events_trader_time` ON `deal_review_events`(`trader_id`);
CREATE INDEX `idx_deal_review_events_user_time` ON `deal_review_events`(`user_id`);
CREATE TABLE `deal_review_ai_scans` (`id` text,`user_id` text NOT NULL,`trader_id` text NOT NULL,`strategy_id` text DEFAULT "",`model_config_id` text DEFAULT "",`provider` text DEFAULT "",`model_name` text DEFAULT "",`dataset_count` integer DEFAULT 0,`filter_json` text DEFAULT "{}",`result_json` text DEFAULT "{}",`strategy_patch_json` text DEFAULT "{}",`status` text DEFAULT "completed",`summary` text DEFAULT "",`error_message` text DEFAULT "",`validation_status` text DEFAULT "pending",`validation_summary` text DEFAULT "",`validation_json` text DEFAULT "{}",`validated_at` datetime,`applied_at` datetime,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_deal_review_ai_scans_trader_time` ON `deal_review_ai_scans`(`trader_id`);
CREATE INDEX `idx_deal_review_ai_scans_user_time` ON `deal_review_ai_scans`(`user_id`);
CREATE INDEX `idx_deal_review_ai_scans_validation` ON `deal_review_ai_scans`(`validation_status`);
CREATE TABLE `deal_review_strategy_versions` (`id` text,`user_id` text NOT NULL,`trader_id` text NOT NULL,`strategy_id` text NOT NULL,`source_scan_id` text DEFAULT "",`source_compare_id` text DEFAULT "",`source_type` text DEFAULT "ai_apply",`summary` text DEFAULT "",`previous_config_json` text DEFAULT "{}",`next_config_json` text DEFAULT "{}",`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_deal_review_strategy_versions_scan` ON `deal_review_strategy_versions`(`source_scan_id`);
CREATE INDEX `idx_deal_review_strategy_versions_strategy` ON `deal_review_strategy_versions`(`strategy_id`);
CREATE INDEX `idx_deal_review_strategy_versions_trader_time` ON `deal_review_strategy_versions`(`trader_id`);
CREATE INDEX `idx_deal_review_strategy_versions_user_time` ON `deal_review_strategy_versions`(`user_id`);
CREATE INDEX `idx_deal_review_strategy_versions_compare` ON `deal_review_strategy_versions`(`source_compare_id`);
CREATE TABLE `deal_review_challenger_compares` (`id` text,`user_id` text NOT NULL,`trader_id` text NOT NULL,`incumbent_trader_id` text NOT NULL,`incumbent_strategy_id` text DEFAULT "",`challenger_trader_id` text DEFAULT "",`challenger_strategy_id` text DEFAULT "",`challenger_exchange_id` text DEFAULT "",`mode` text DEFAULT "shared_live",`window_hours` integer DEFAULT 24,`extension_count` integer DEFAULT 0,`source_scan_id` text DEFAULT "",`source_strategy_version_id` text DEFAULT "",`status` text DEFAULT "starting",`winner_trader_id` text DEFAULT "",`loser_trader_id` text DEFAULT "",`summary` text DEFAULT "",`error_message` text DEFAULT "",`metrics_json` text DEFAULT "{}",`protocol_json` text DEFAULT "[]",`started_at` datetime,`ends_at` datetime,`last_evaluated_at` datetime,`resolved_at` datetime,`created_at` datetime,`updated_at` datetime,PRIMARY KEY (`id`));
CREATE INDEX `idx_deal_review_challenger_compares_user_time` ON `deal_review_challenger_compares`(`user_id`);
CREATE INDEX `idx_deal_review_challenger_compares_trader_time` ON `deal_review_challenger_compares`(`trader_id`);
CREATE INDEX `idx_deal_review_challenger_compares_incumbent` ON `deal_review_challenger_compares`(`incumbent_trader_id`);
CREATE INDEX `idx_deal_review_challenger_compares_challenger` ON `deal_review_challenger_compares`(`challenger_trader_id`);
CREATE INDEX `idx_deal_review_challenger_compares_scan` ON `deal_review_challenger_compares`(`source_scan_id`);
CREATE INDEX `idx_deal_review_challenger_compares_status` ON `deal_review_challenger_compares`(`status`);
CREATE INDEX `idx_deal_review_challenger_compares_started` ON `deal_review_challenger_compares`(`started_at`);
CREATE INDEX `idx_deal_review_challenger_compares_ends` ON `deal_review_challenger_compares`(`ends_at`);
CREATE TABLE `deal_review_cycle_points` (`id` integer PRIMARY KEY AUTOINCREMENT,`user_id` text NOT NULL,`trader_id` text NOT NULL,`deal_id` text DEFAULT "",`position_id` integer NOT NULL,`symbol` text NOT NULL,`side` text NOT NULL,`timestamp_ms` integer NOT NULL,`decision_cycle_number` integer NOT NULL,`mark_price` real DEFAULT 0,`entry_price` real DEFAULT 0,`quantity` real DEFAULT 0,`unrealized_pnl` real DEFAULT 0,`unrealized_pnl_pct` real DEFAULT 0,`in_profit` numeric DEFAULT false,`created_at` datetime,`updated_at` datetime);
CREATE INDEX `idx_deal_review_cycle_points_user_time` ON `deal_review_cycle_points`(`user_id`);
CREATE INDEX `idx_deal_review_cycle_points_trader_time` ON `deal_review_cycle_points`(`trader_id`);
CREATE INDEX `idx_deal_review_cycle_points_deal` ON `deal_review_cycle_points`(`deal_id`);
CREATE INDEX `idx_deal_review_cycle_points_position` ON `deal_review_cycle_points`(`position_id`);
CREATE UNIQUE INDEX `idx_deal_review_cycle_points_position_cycle` ON `deal_review_cycle_points`(`position_id`,`decision_cycle_number`);
CREATE INDEX `idx_deal_review_cycle_points_symbol` ON `deal_review_cycle_points`(`symbol`);
CREATE INDEX `idx_deal_review_cycle_points_time` ON `deal_review_cycle_points`(`timestamp_ms`);
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
