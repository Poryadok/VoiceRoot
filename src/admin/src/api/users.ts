import { apiJson } from "./client";

interface ProfileResponse {
  profile?: {
    account_id?: string;
  };
}

export async function resolveAccountIdForProfile(
  profileId: string,
): Promise<string | undefined> {
  try {
    const data = await apiJson<ProfileResponse>(
      `/api/v1/users/profiles/${encodeURIComponent(profileId)}`,
    );
    return data.profile?.account_id;
  } catch {
    return undefined;
  }
}
