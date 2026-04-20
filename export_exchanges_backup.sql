PRAGMA foreign_keys=OFF;
BEGIN TRANSACTION;
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
INSERT INTO exchanges VALUES('binance','default','Binance Futures','binance',0,'','',0,'','','','','2025-11-07 18:23:38','2025-11-07 18:23:38');
INSERT INTO exchanges VALUES('aster','default','Aster DEX','aster',0,'','',0,'','','','','2025-11-07 18:23:38','2025-11-07 18:23:38');
INSERT INTO exchanges VALUES('binance','admin','Binance Futures','cex',1,'sH9vKQKv4NngdcddH2Cw9ojP91m42dyHH2X0F2yW0QlUa7j8BC6J5qppTrFoBnxI','GBAAniqVKcCaBYV0VMckCHnMGVJt4SIz6JhHb0udBLMyUHjYuGYOP8L2Molr4TwV',0,'','','','','2025-11-07 18:37:58','2025-11-07 21:52:04');
INSERT INTO exchanges VALUES('hyperliquid','default','Hyperliquid','hyperliquid',0,'','',0,'','','','','2025-11-08 12:58:03','2025-11-08 12:58:03');
INSERT INTO exchanges VALUES('hyperliquid','7a250f1b-f005-45ee-87cc-be1dfdda8079','Hyperliquid','hyperliquid',1,'placeholder-private-key','',0,'0xc6EdA1C262f23cBA825732C530789d86CA2D1029','','','','2025-11-08 15:19:16','2025-11-08 15:19:16');
INSERT INTO exchanges VALUES('hyperliquid','Test','Hyperliquid','hyperliquid',1,'ENC:v1:lXSZQJhZhLU2D1iJ:vvVrA3ohSfqlakp68MHYNE23uUWzCeDoNAe8yjHHyWCViqJ/OqehCpH2NDXMJAmQxdVbJbQCMQ72ubQRn+lalLOBpyc1CehQcHePMMHHMfIbYg==','',0,'0xc6EdA1C262f23cBA825732C530789d86CA2D1029','','','','2025-11-08 15:45:41','2025-11-09 10:51:25');
COMMIT;
