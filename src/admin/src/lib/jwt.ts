export function decodeJwtPayload(token: string): Record<string, unknown> | null {
  const parts = token.split(".");
  if (parts.length < 2) {
    return null;
  }
  try {
    const normalized = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(
      normalized.length + ((4 - (normalized.length % 4)) % 4),
      "=",
    );
    const json = atob(padded);
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return null;
  }
}

export function staffProfileIdFromToken(): string | undefined {
  const token = import.meta.env.VITE_STAFF_TOKEN;
  if (!token) {
    return undefined;
  }
  const payload = decodeJwtPayload(token);
  const profileId = payload?.profile_id ?? payload?.profileId;
  return typeof profileId === "string" && profileId.length > 0
    ? profileId
    : undefined;
}
