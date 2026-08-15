import { decodeJwtPayload } from "./decodeJwtPayload";
import { getAccessToken } from "../oauth/session";

export function staffProfileIdFromToken(): string | undefined {
  const sessionToken = getAccessToken();
  const envToken = import.meta.env.VITE_STAFF_TOKEN;
  const token = sessionToken ?? envToken;
  if (!token) {
    return undefined;
  }
  const payload = decodeJwtPayload(token);
  const profileId = payload?.profile_id ?? payload?.profileId;
  return typeof profileId === "string" && profileId.length > 0
    ? profileId
    : undefined;
}
