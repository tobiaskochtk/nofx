# Field Naming Compatibility Guide

## Übersicht

Das NOFX Trading System unterstützt jetzt **beide** Field-Naming-Konventionen für maximale Flexibilität:

### ✅ Unterstützte Formate

| Old Format (Standard) | New Format (v3.1 Payload) | Beschreibung |
|----------------------|---------------------------|--------------|
| `symbol` | `sym` | Trading-Paar Symbol |
| `leverage` | `lev` | Hebel (3-15x) |
| `position_size_usd` | `size_pct` | Positionsgröße (USD vs Prozent) |
| `reasoning` | `reason_codes` | Begründung (Text vs Array) |
| `stop_loss`, `take_profit` | `stops_targets: {sl, tp}` | Stop Loss & Take Profit |

---

## 🔧 Technische Implementierung

### Go-Code (engine.go)

Die `Decision` Struktur hat jetzt eine erweiterte `UnmarshalJSON` Methode:

```go
func (d *Decision) UnmarshalJSON(data []byte) error {
    // Unterstützt BEIDE Feldnamen:
    // - sym → symbol
    // - lev → leverage
    // - size_pct → position_size_usd (konvertiert mit Account Equity)
    // - reason_codes → reasoning (joined)
    // - stops_targets → stop_loss/take_profit
}
```

### Automatische Konvertierung

1. **Symbol Mapping**: `sym` → `symbol`
2. **Leverage Mapping**: `lev` → `leverage`
3. **Size Conversion**: `size_pct` → `position_size_usd`
   - Wird in `validateDecisions` konvertiert
   - Berechnung: `position_size_usd = accountEquity * size_pct`
4. **Reasoning Join**: `["code1", "code2"]` → `"code1, code2"`
5. **Stops Extraction**: `{sl: 100, tp: 200}` → `stop_loss: 100, take_profit: 200`

---

## 📝 Prompt Templates

### Option 1: Standard Format (Empfohlen)

**Datei**: `prompts/default.txt` oder `prompts/v3_compatible.txt`

**Output Format**:
```json
{
  "decisions": [
    {
      "symbol": "BTCUSDT",
      "action": "open_long",
      "leverage": 5,
      "position_size_usd": 2200,
      "stop_loss": 97000,
      "take_profit": 101000,
      "confidence": 85,
      "reasoning": "Strong breakout with volume confirmation"
    }
  ],
  "notes": "Bullish market trend"
}
```

**Vorteile**:
- ✅ Direkt kompatibel mit Go-Strukturen
- ✅ Keine Konvertierung nötig
- ✅ Bewährt und stabil
- ✅ Einfach zu debuggen

### Option 2: v3.1 Payload Format

**Output Format**:
```json
{
  "decisions": [
    {
      "sym": "BTCUSDT",
      "action": "open_long",
      "lev": 5,
      "size_pct": 0.23,
      "stops_targets": {"sl": 97000, "tp": 101000},
      "reason_codes": ["bull_div_s", "dist_ok", "conf_ok"]
    }
  ],
  "notes": "Bullish market trend"
}
```

**Vorteile**:
- ✅ Kompakt und konsistent mit Payload-Format
- ✅ Automatische Konvertierung
- ✅ Weniger Zeichen (Token-Effizienz)

**Nachteile**:
- ⚠️ Benötigt Account Equity für size_pct Konvertierung
- ⚠️ Zusätzlicher Verarbeitungsschritt

---

## 🚀 Verwendung

### In der Datenbank (Custom Prompt)

**Standard Format verwenden**:
```sql
UPDATE traders 
SET custom_prompt = (SELECT content FROM file('prompts/v3_compatible.txt'))
WHERE id = 'your-trader-id';
```

**Oder v3.1 Format**:
- Das System erkennt automatisch beide Formate
- Keine Änderung nötig, solange die Felder korrekt sind

### Testing

```bash
# Kompilieren mit erweiterter Unterstützung
go build -o nofx.exe .

# Testen mit beiden Formaten
# Das System akzeptiert automatisch beide
```

---

## 🔍 Debugging

### Log-Ausgaben

Bei der Konvertierung von `size_pct` siehst du:
```
✓ Converted size_pct 23.00% to position_size_usd 2162.00 USD (equity: 9400.00)
```

Bei fehlerhaften Feldern:
```
❌ 决策 #1 验证失败: symbol ist leer
❌ 决策 #1 验证失败: 杠杆必须大于0: 0
```

### Häufige Probleme

1. **Leeres Symbol**:
   - Ursache: AI hat `""` ausgegeben
   - Lösung: Prompt klarer formulieren

2. **Leverage = 0**:
   - Ursache: Weder `leverage` noch `lev` gesetzt
   - Lösung: Im Prompt explizit fordern

3. **Position Size = 0**:
   - Ursache: Weder `position_size_usd` noch `size_pct` gesetzt
   - Lösung: Beispiele im Prompt zeigen

---

## ✅ Best Practices

1. **Verwende Standard Format für Production**
   - Bewährt und stabil
   - Keine Konvertierung nötig
   - Einfacher zu debuggen

2. **Verwende v3.1 Format für Token-Effizienz**
   - Bei API-Kosten-Optimierung
   - Wenn Payload-Konsistenz wichtig ist

3. **Validierung ist key**
   - Immer alle Pflichtfelder prüfen
   - Log-Ausgaben beachten
   - Tests mit beiden Formaten

4. **Prompt-Qualität**
   - Klare Beispiele zeigen
   - Alle Pflichtfelder explizit fordern
   - Fehlerbehandlung erklären

---

## 📊 Migration

### Von altem zu neuem Format

Keine Migration nötig! Beide Formate werden parallel unterstützt.

### Empfohlener Workflow

1. **Bestehende Systeme**: Behalten Standard Format
2. **Neue Features**: Können v3.1 Format nutzen
3. **Testing**: Beide Formate in Staging testen
4. **Production**: Standard Format für Stabilität

---

## 🐛 Troubleshooting

### Problem: AI gibt leere Felder aus

**Symptom**:
```json
{"symbol": "", "action": "hold", "reasoning": ""}
```

**Lösung**:
- Prompt klarer formulieren
- Beispiele mit vollständigen Daten zeigen
- System Prompt vereinfachen

### Problem: size_pct wird nicht konvertiert

**Symptom**:
```
✓ Converted size_pct 0.23% to position_size_usd 21.62 USD
```

**Ursache**: size_pct als absolute Zahl statt Prozent

**Lösung**: Im Prompt klarstellen: `size_pct: 0.23` = 23% of equity

---

## 📞 Support

Bei Fragen oder Problemen:
1. Check Decision Logs: `decision_logs/*/decision_*.json`
2. Check System Logs: Look for conversion messages
3. Verify Prompt Template: Ensure all required fields are specified
4. Test with both formats to isolate issue

---

**Version**: 1.0 (Compatible Mode)  
**Last Updated**: 2025-11-12  
**Status**: ✅ Production Ready
