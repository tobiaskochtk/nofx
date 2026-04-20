package trader

import "nofx/mcp"

func (at *AutoTrader) setAICallerContext(component string, cycleNumber int) {
	if at == nil || at.mcpClient == nil {
		return
	}
	mcp.SetCallerContext(at.mcpClient, mcp.CallerContext{
		CallerType:  "trader",
		CallerID:    at.id,
		CallerName:  at.name,
		UserID:      at.userID,
		TraderID:    at.id,
		TraderName:  at.name,
		Component:   component,
		CycleNumber: cycleNumber,
	})
}
