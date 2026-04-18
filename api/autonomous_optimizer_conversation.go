package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nofx/mcp"
	"nofx/store"
)

const autonomousOptimizerConversationReplayCharLimit = 2200

func flattenAutonomousOptimizerJSONPaths(value map[string]any) []string {
	if len(value) == 0 {
		return nil
	}
	paths := make([]string, 0, 16)
	var walk func(prefix string, current any)
	walk = func(prefix string, current any) {
		switch typed := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				nextPrefix := key
				if prefix != "" {
					nextPrefix = prefix + "." + key
				}
				walk(nextPrefix, typed[key])
			}
		case []any:
			if len(typed) > 0 && prefix != "" {
				paths = append(paths, prefix)
			}
		case nil:
			return
		default:
			if prefix != "" {
				paths = append(paths, prefix)
			}
		}
	}
	walk("", value)
	return dedupeSortedStrings(paths)
}

func clipAutonomousOptimizerConversationReplay(value string) string {
	return clipDealReviewAIScanText(strings.TrimSpace(value), autonomousOptimizerConversationReplayCharLimit)
}

func buildAutonomousOptimizerConversationReplay(kind string, payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	header := strings.TrimSpace(kind)
	if header == "" {
		header = "optimizer_memory"
	}
	return clipAutonomousOptimizerConversationReplay(header + "\n" + string(body))
}

func buildAutonomousOptimizerProposalConversationUserReplay(payload map[string]any) string {
	replay := map[string]any{
		"kind":                        "proposal_request",
		"review_window":               parseAutonomousOptimizerNestedObject(payload, "review_window"),
		"dataset_summary":             parseAutonomousOptimizerNestedObject(payload, "dataset_summary"),
		"decision_starvation_metrics": parseAutonomousOptimizerNestedObject(payload, "decision_starvation_metrics"),
		"latest_gate_feedback":        parseAutonomousOptimizerNestedObject(payload, "latest_gate_feedback"),
	}
	if telemetry := parseAutonomousOptimizerNestedObject(payload, "adaptive_cooldown_telemetry"); len(telemetry) > 0 {
		replay["adaptive_cooldown_telemetry"] = map[string]any{
			"cooldown_candidate_count":        telemetry["cooldown_candidate_count"],
			"same_session_reentry_count":      telemetry["same_session_reentry_count"],
			"repeat_after_loss_count":         telemetry["repeat_after_loss_count"],
			"regime_repeat_loss_count":        telemetry["regime_repeat_loss_count"],
			"blocked_symbol_reentry_outcomes": telemetry["blocked_symbol_reentry_outcomes"],
			"post_symbol_cooldown_outcomes":   telemetry["post_symbol_cooldown_outcomes"],
			"blocked_regime_reentry_outcomes": telemetry["blocked_regime_reentry_outcomes"],
			"post_regime_cooldown_outcomes":   telemetry["post_regime_cooldown_outcomes"],
			"top_symbols":                     limitAutonomousOptimizerReplayArray(telemetry["top_symbols"], 3),
			"top_regimes":                     limitAutonomousOptimizerReplayArray(telemetry["top_regimes"], 3),
		}
	}
	if telemetry := parseAutonomousOptimizerNestedObject(payload, "trailing_stop_telemetry"); len(telemetry) > 0 {
		replay["trailing_stop_telemetry"] = map[string]any{
			"trailing_exit_count":         telemetry["trailing_exit_count"],
			"trailing_profit_exit_count":  telemetry["trailing_profit_exit_count"],
			"trailing_loss_exit_count":    telemetry["trailing_loss_exit_count"],
			"early_tightening_count":      telemetry["early_tightening_count"],
			"early_tightening_loss_count": telemetry["early_tightening_loss_count"],
		}
	}
	return buildAutonomousOptimizerConversationReplay("proposal_request_memory", replay)
}

func buildAutonomousOptimizerProposalConversationAssistantReplay(proposal *autonomousOptimizerProposalResult) string {
	if proposal == nil {
		return ""
	}
	replay := map[string]any{
		"kind":              "proposal_response",
		"proposal_type":     proposal.ProposalType,
		"executive_summary": proposal.ExecutiveSummary,
		"expected_effect":   proposal.ExpectedEffect,
		"confidence":        roundAutonomousOptimizerFloat(proposal.Confidence, 3),
		"evidence_strength": roundAutonomousOptimizerFloat(proposal.EvidenceStrength, 3),
		"rationale":         limitAutonomousOptimizerReplayStrings(proposal.Rationale, 6),
		"config_patch_paths": flattenAutonomousOptimizerJSONPaths(
			sanitizeAutonomousOptimizerJSONMap(proposal.ConfigPatch),
		),
		"prompt_patch_fields": flattenAutonomousOptimizerJSONPaths(
			sanitizeAutonomousOptimizerJSONMap(proposal.PromptPatch),
		),
		"backlog_titles": limitAutonomousOptimizerBacklogTitles(
			proposal.BacklogItems,
			4,
		),
	}
	return buildAutonomousOptimizerConversationReplay("proposal_response_memory", replay)
}

func buildAutonomousOptimizerCriticConversationUserReplay(payload map[string]any, proposal *autonomousOptimizerProposalResult) string {
	replay := map[string]any{
		"kind":                 "critic_request",
		"review_window":        parseAutonomousOptimizerNestedObject(payload, "review_window"),
		"latest_gate_feedback": parseAutonomousOptimizerNestedObject(payload, "latest_gate_feedback"),
	}
	if proposal != nil {
		replay["proposal"] = map[string]any{
			"proposal_type":      proposal.ProposalType,
			"executive_summary":  proposal.ExecutiveSummary,
			"expected_effect":    proposal.ExpectedEffect,
			"confidence":         roundAutonomousOptimizerFloat(proposal.Confidence, 3),
			"evidence_strength":  roundAutonomousOptimizerFloat(proposal.EvidenceStrength, 3),
			"config_patch_paths": flattenAutonomousOptimizerJSONPaths(sanitizeAutonomousOptimizerJSONMap(proposal.ConfigPatch)),
			"prompt_patch_fields": flattenAutonomousOptimizerJSONPaths(
				sanitizeAutonomousOptimizerJSONMap(proposal.PromptPatch),
			),
		}
	}
	return buildAutonomousOptimizerConversationReplay("critic_request_memory", replay)
}

func buildAutonomousOptimizerCriticConversationAssistantReplay(critic *autonomousOptimizerCriticResult) string {
	if critic == nil {
		return ""
	}
	replay := map[string]any{
		"kind":               "critic_response",
		"approved":           critic.Approved,
		"recommended_action": critic.RecommendedAction,
		"confidence":         roundAutonomousOptimizerFloat(critic.Confidence, 3),
		"summary":            critic.Summary,
		"blocking_issues":    limitAutonomousOptimizerReplayStrings(critic.BlockingIssues, 6),
		"warnings":           limitAutonomousOptimizerReplayStrings(critic.Warnings, 6),
	}
	return buildAutonomousOptimizerConversationReplay("critic_response_memory", replay)
}

func limitAutonomousOptimizerReplayArray(value any, limit int) []any {
	if limit <= 0 {
		return nil
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func limitAutonomousOptimizerReplayStrings(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func limitAutonomousOptimizerBacklogTitles(items []autonomousOptimizerBacklogProposal, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	titles := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item.Title); trimmed != "" {
			titles = append(titles, trimmed)
		}
	}
	return titles
}

func buildAutonomousOptimizerConversationRequest(history []*store.AutonomousOptimizerConversationMessage, replayLimit int, systemPrompt, userPrompt string) (*mcp.Request, int, error) {
	builder := mcp.NewRequestBuilder().AddSystemMessage(systemPrompt)
	if replayLimit <= 0 {
		replayLimit = store.AutonomousOptimizerDefaultConversationReplayMessages
	}
	replayable := make([]*store.AutonomousOptimizerConversationMessage, 0, len(history))
	for _, item := range history {
		if item == nil || strings.TrimSpace(item.ReplayContent) == "" {
			continue
		}
		replayable = append(replayable, item)
	}
	if len(replayable) > replayLimit {
		replayable = replayable[len(replayable)-replayLimit:]
	}
	replayed := 0
	for _, item := range replayable {
		if item == nil {
			continue
		}
		replay := strings.TrimSpace(item.ReplayContent)
		if replay == "" {
			continue
		}
		builder.AddMessage(item.Role, replay)
		replayed++
	}
	builder.AddUserMessage(userPrompt)
	req, err := builder.Build()
	if err != nil {
		return nil, 0, err
	}
	return req, replayed, nil
}

func persistAutonomousOptimizerConversationTurn(s *Server, conversation *store.AutonomousOptimizerConversation, runID, systemPrompt, userPrompt, assistantResponse, userReplay, assistantReplay string) error {
	if s == nil || s.store == nil || conversation == nil {
		return fmt.Errorf("autonomous optimizer conversation context is incomplete")
	}
	messages := []*store.AutonomousOptimizerConversationMessage{
		{
			Role:          store.AutonomousOptimizerConversationRoleSystem,
			Content:       strings.TrimSpace(systemPrompt),
			ReplayContent: "",
		},
		{
			Role:          store.AutonomousOptimizerConversationRoleUser,
			Content:       strings.TrimSpace(userPrompt),
			ReplayContent: strings.TrimSpace(userReplay),
		},
		{
			Role:          store.AutonomousOptimizerConversationRoleAssistant,
			Content:       strings.TrimSpace(assistantResponse),
			ReplayContent: strings.TrimSpace(assistantReplay),
		},
	}
	return s.store.AutonomousOptimizer().AppendConversationMessages(conversation, runID, messages)
}
