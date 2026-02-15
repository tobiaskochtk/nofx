.mode insert exchanges
.output export_exchanges_backup.sql
SELECT * FROM exchanges;
.output stdout
