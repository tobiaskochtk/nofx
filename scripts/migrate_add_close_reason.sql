-- Migration: Add close_reason column to deals table
-- Date: 2025-11-14
-- Description: Adds close_reason TEXT column to track why a deal was closed

-- Add close_reason column if it doesn't exist
ALTER TABLE deals ADD COLUMN close_reason TEXT;

-- Update existing closed deals with default reason
UPDATE deals SET close_reason = 'unknown' WHERE status = 'closed' AND close_reason IS NULL;

-- Show results
SELECT COUNT(*) as total_deals, 
       COUNT(close_reason) as deals_with_reason,
       COUNT(*) - COUNT(close_reason) as deals_without_reason
FROM deals;
