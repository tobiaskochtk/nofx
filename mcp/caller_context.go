package mcp

import "strings"

// CallerContext carries the logical source of an AI request so provider-side
// logging can attribute real transport calls to traders and other subsystems.
type CallerContext struct {
	CallerType  string
	CallerID    string
	CallerName  string
	UserID      string
	TraderID    string
	TraderName  string
	Component   string
	CycleNumber int
}

func (c CallerContext) Normalized() CallerContext {
	c.CallerType = strings.TrimSpace(c.CallerType)
	c.CallerID = strings.TrimSpace(c.CallerID)
	c.CallerName = strings.TrimSpace(c.CallerName)
	c.UserID = strings.TrimSpace(c.UserID)
	c.TraderID = strings.TrimSpace(c.TraderID)
	c.TraderName = strings.TrimSpace(c.TraderName)
	c.Component = strings.TrimSpace(c.Component)
	if c.CycleNumber < 0 {
		c.CycleNumber = 0
	}
	return c
}

// SetCallerContext attaches source metadata to an AI client when the concrete
// implementation embeds the base *Client.
func SetCallerContext(client AIClient, caller CallerContext) {
	embedder, ok := client.(ClientEmbedder)
	if !ok {
		return
	}
	base := embedder.BaseClient()
	if base == nil {
		return
	}
	base.Caller = caller.Normalized()
}

// GetCallerContext returns the current caller metadata for an AI client.
func GetCallerContext(client AIClient) CallerContext {
	embedder, ok := client.(ClientEmbedder)
	if !ok {
		return CallerContext{}
	}
	base := embedder.BaseClient()
	if base == nil {
		return CallerContext{}
	}
	return base.Caller.Normalized()
}
