export function buildResolutionJson(note?: string): string {
  const trimmed = note?.trim() ?? "";
  if (!trimmed) {
    return "{}";
  }
  return JSON.stringify({ note: trimmed });
}
