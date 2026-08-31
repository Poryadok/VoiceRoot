/** Platform temp bans: 1–30 days per docs/features/reports.md */
export const TEMP_BAN_MIN_DAYS = 1;
export const TEMP_BAN_MAX_DAYS = 30;
export const TEMP_BAN_DEFAULT_DAYS = 7;

export function tempBanExpiresAtIso(days: number, now = new Date()): string {
  const clamped = Math.min(
    TEMP_BAN_MAX_DAYS,
    Math.max(TEMP_BAN_MIN_DAYS, Math.floor(days)),
  );
  const expires = new Date(now.getTime());
  expires.setUTCDate(expires.getUTCDate() + clamped);
  return expires.toISOString();
}
