-- The original key used "meeting-cleanup-" + UUID (52 chars), but the
-- delivery table intentionally limits Feishu's uuid/idempotency value to 50.
-- Backfill deliveries that were silently skipped because of that constraint.
INSERT INTO meeting_booking_deliveries (
  id,
  booking_id,
  scheduled_for,
  status,
  attempts,
  next_attempt_at,
  message_id,
  error,
  idempotency_key,
  created_at,
  updated_at
)
SELECT
  gen_random_uuid(),
  b.id,
  b.end_at,
  CASE WHEN b.end_at < now() - interval '30 minutes' THEN 'failed' ELSE 'pending' END,
  0,
  b.end_at,
  '',
  CASE WHEN b.end_at < now() - interval '30 minutes'
    THEN '会后提醒任务修复时已超过补发窗口，未向用户发送过期提醒'
    ELSE ''
  END,
  'mtg-clean-' || replace(b.id::text, '-', ''),
  now(),
  now()
FROM meeting_bookings b
WHERE b.status = 'active'
  AND NOT EXISTS (
    SELECT 1
    FROM meeting_booking_deliveries d
    WHERE d.booking_id = b.id
  )
ON CONFLICT (booking_id, scheduled_for) DO NOTHING;
