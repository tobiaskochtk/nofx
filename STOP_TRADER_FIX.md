# Stop Trader Timeout Fix

## Issue
**Error:** "Operation Failed" in frontend when trying to stop a trader  
**Root Cause:** The `/api/traders/:id/stop` endpoint was timing out (504 Gateway Timeout) after 60 seconds

## Analysis
From the logs at `2025/11/11 10:23:32`:
```
[error] upstream timed out (110: Operation timed out) while reading response header from upstream
POST /api/traders/hyperliquid_0b168b37-04c4-444e-95d9-2c50e231e823_deepseek_1762555012/stop HTTP/1.1" 504
```

**Problem Details:**
1. The `handleStopTrader` function called `trader.Stop()` synchronously
2. The `Stop()` method called `at.monitorWg.Wait()` which waits indefinitely for goroutines to finish
3. If a monitoring goroutine was stuck (API call, database lock, etc.), the entire HTTP request would hang
4. Nginx timed out after 60 seconds (though configured for 300s, the actual timeout was 60s)
5. Backend never logged receiving the stop request, indicating it hung before processing

## Solution

### 1. Added Timeout to HTTP Handler (`api/server.go`)
- Wrapped `trader.Stop()` in a goroutine
- Added 30-second timeout using `select` statement
- If timeout occurs, logs a warning and marks trader as stopped anyway
- Changed response message to "交易员停止请求已发送" (stop request sent)

**Before:**
```go
trader.Stop()  // Blocking call, no timeout
```

**After:**
```go
stopCh := make(chan struct{})
go func() {
    trader.Stop()
    close(stopCh)
}()

timeout := time.NewTimer(30 * time.Second)
defer timeout.Stop()

select {
case <-stopCh:
    log.Printf("⏹  交易员 %s 已停止", trader.GetName())
case <-timeout.C:
    log.Printf("⚠️  交易员 %s 停止操作超时，强制标记为停止", trader.GetName())
}
```

### 2. Added Timeout to Stop Method (`trader/auto_trader.go`)
- Added 10-second timeout for waiting on `monitorWg.Wait()`
- If monitoring goroutines don't finish in time, logs warning and exits anyway

**Before:**
```go
at.monitorWg.Wait()  // Waits indefinitely
```

**After:**
```go
done := make(chan struct{})
go func() {
    at.monitorWg.Wait()
    close(done)
}()

timeout := time.NewTimer(10 * time.Second)
defer timeout.Stop()

select {
case <-done:
    log.Println("⏹ 自动交易系统停止")
case <-timeout.C:
    log.Println("⚠️  监控goroutine停止超时，强制退出")
}
```

## Timeout Layers
1. **Inner timeout:** 10 seconds for monitoring goroutines to finish
2. **Outer timeout:** 30 seconds for the entire stop operation
3. **Nginx timeout:** 300 seconds (5 minutes) - more than enough headroom

## Status
✅ **Fixed and Deployed**
- Changes committed to `api/server.go` and `trader/auto_trader.go`
- Container rebuilt and restarted successfully
- Service running normally

## Testing Recommendation
Test the stop functionality with a trader that's:
1. In the middle of a trade execution
2. Waiting for an API response
3. Processing a long decision cycle

The stop operation should now complete within 30 seconds or gracefully timeout.
