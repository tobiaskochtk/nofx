package kernel

import (
	"fmt"
	"strings"
	"time"

	"nofx/market"
	"nofx/pkg/types"
)

func formatDerivsSignals(data *market.Data) string {
	if data == nil || data.Snapshot == nil || data.Snapshot.Features.Derivs == nil {
		return ""
	}

	d := data.Snapshot.Features.Derivs
	var sb strings.Builder
	sb.WriteString("Derivs Snapshot (base + f4-f7):\n")

	if line := formatDerivsBaseLine(d); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if line := formatFeature4Line(d); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if line := formatFeature5Line(d); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if line := formatFeature6Line(d); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if line := formatFeature7Line(d); line != "" {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("QoS: f4[%s] | f5[%s] | f6[%s] | f7[%s]\n",
		formatFeatureQoS(data, market.FeatureKeyF4),
		formatFeatureQoS(data, market.FeatureKeyF5),
		formatFeatureQoS(data, market.FeatureKeyF6),
		formatFeatureQoS(data, market.FeatureKeyF7),
	))

	return sb.String()
}

func formatDerivsBaseLine(d *types.DerivsFeatures) string {
	parts := make([]string, 0, 10)
	addFloatPart(&parts, "oi_delta_1h_pct", d.OIDelta1hPct, 100)
	addFloatPart(&parts, "oi_z_7d", d.OIZ7d, 1)
	addStringPart(&parts, "oi_div", d.OIPriceDiv)
	addFloatPart(&parts, "oi_corr_24h", d.OIPriceCorr24h, 1)
	addFloatPart(&parts, "fund_bps", d.FundingLatestBps, 1)
	addFloatPart(&parts, "fund_z_7d", d.FundingMedianZ7d, 1)
	addFloatPart(&parts, "fund_disp_bps", d.FundingDispersionBps, 1)
	addFloatPart(&parts, "basis_pct", d.BasisPct, 100)
	addFloatPart(&parts, "basis_z_14d", d.BasisZ14d, 1)
	if len(parts) == 0 {
		return ""
	}
	return "DerivsBase: " + strings.Join(parts, " | ")
}

func formatFeature4Line(d *types.DerivsFeatures) string {
	parts := make([]string, 0, 16)
	addFloatPart(&parts, "cvd_s", d.CVDNotionalZ3mShort, 1)
	addFloatPart(&parts, "cvd_l", d.CVDNotionalZ3mLong, 1)
	addFloatPart(&parts, "imb_s", d.ImbNotionalZ3mShort, 1)
	addFloatPart(&parts, "imb_l", d.ImbNotionalZ3mLong, 1)
	addFloatPart(&parts, "tbr", d.TBRNotional3m, 1)
	addFloatPart(&parts, "p_slope_s", d.SlopePrice3mShort, 1)
	addFloatPart(&parts, "cvd_slope_s", d.SlopeCVDZ3mShort, 1)
	addIntPart(&parts, "div_bear_s", d.DivBearShort3m)
	addIntPart(&parts, "div_bear_m", d.DivBearMid3m)
	addFloatPart(&parts, "conf", d.ConfidenceCVD3m, 1)
	addFloatPart(&parts, "c15_conf", d.ConfidenceCVD15m, 1)
	if len(parts) == 0 {
		return ""
	}
	return "F4(CVD): " + strings.Join(parts, " | ")
}

func formatFeature5Line(d *types.DerivsFeatures) string {
	parts := make([]string, 0, 12)
	addFloatPart(&parts, "up_atr", d.DistUpAtr3m, 1)
	addFloatPart(&parts, "dn_atr", d.DistDnAtr3m, 1)
	addStringPart(&parts, "prefer", d.PreferDirection3m)
	addIntPart(&parts, "risk_up", d.LiqRiskUp3m)
	addIntPart(&parts, "risk_dn", d.LiqRiskDown3m)
	addFloatPart(&parts, "conf", d.ConfidenceLiq3m, 1)
	addFloatPart(&parts, "c15_up_atr", d.DistUpAtr15m, 1)
	addFloatPart(&parts, "c15_dn_atr", d.DistDnAtr15m, 1)
	addStringPart(&parts, "c15_prefer", d.PreferDirection15m)
	addFloatPart(&parts, "c15_conf", d.ConfidenceLiq15m, 1)
	if len(parts) == 0 {
		return ""
	}
	return "F5(Liq): " + strings.Join(parts, " | ")
}

func formatFeature6Line(d *types.DerivsFeatures) string {
	parts := make([]string, 0, 12)
	addStringPart(&parts, "up_name", d.AVWAPUpName3m)
	addFloatPart(&parts, "up_atr", d.AVWAPUpDistAtr3m, 1)
	addStringPart(&parts, "dn_name", d.AVWAPDnName3m)
	addFloatPart(&parts, "dn_atr", d.AVWAPDnDistAtr3m, 1)
	addStringPart(&parts, "bias", d.AVWAPBias3m)
	addIntPart(&parts, "reclaim_up", d.AVWAPReclaimUp3m)
	addIntPart(&parts, "reject_dn", d.AVWAPRejectionDn3m)
	addFloatPart(&parts, "conf", d.ConfidenceAVWAP3m, 1)
	addStringPart(&parts, "c15_bias", d.AVWAPBias15m)
	addFloatPart(&parts, "c15_conf", d.ConfidenceAVWAP15m, 1)
	if len(parts) == 0 {
		return ""
	}
	return "F6(AVWAP): " + strings.Join(parts, " | ")
}

func formatFeature7Line(d *types.DerivsFeatures) string {
	parts := make([]string, 0, 10)
	addFloatPart(&parts, "bbw", d.BBW3m, 1)
	addIntPart(&parts, "sq", d.SqueezeOn3m)
	addFloatPart(&parts, "rv", d.RvRatio3m, 1)
	addStringPart(&parts, "reg", d.VolRegime3m)
	addFloatPart(&parts, "conf", d.ConfidenceVol3m, 1)
	addIntPart(&parts, "c15_sq", d.SqueezeOn15m)
	addFloatPart(&parts, "c15_rv", d.RvRatio15m, 1)
	addStringPart(&parts, "c15_reg", d.VolRegime15m)
	addFloatPart(&parts, "c15_conf", d.ConfidenceVol15m, 1)
	if len(parts) == 0 {
		return ""
	}
	return "F7(Vol): " + strings.Join(parts, " | ")
}

func formatFeatureQoS(data *market.Data, key string) string {
	stat, ok := data.FeatureQuality(key)
	if !ok {
		return "n/a"
	}
	age := 0
	if !stat.UpdatedAt.IsZero() {
		age = int(time.Since(stat.UpdatedAt).Seconds())
		if age < 0 {
			age = 0
		}
	}
	return fmt.Sprintf("cov=%.2f age_s=%d", stat.Coverage, age)
}

func addFloatPart(parts *[]string, key string, value *float64, multiplier float64) {
	if value == nil {
		return
	}
	v := *value
	if multiplier != 0 && multiplier != 1 {
		v *= multiplier
	}
	*parts = append(*parts, fmt.Sprintf("%s=%.4f", key, v))
}

func addIntPart(parts *[]string, key string, value *int) {
	if value == nil {
		return
	}
	*parts = append(*parts, fmt.Sprintf("%s=%d", key, *value))
}

func addStringPart(parts *[]string, key string, value *string) {
	if value == nil {
		return
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return
	}
	*parts = append(*parts, fmt.Sprintf("%s=%s", key, strings.ToLower(v)))
}
