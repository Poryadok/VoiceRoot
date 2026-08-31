import { apiJson } from "../api/client";
import type { Report } from "../api/types";
import { resolveAccountIdForProfile } from "../api/users";

interface SpaceResponse {
  space?: {
    owner_profile_id?: string;
  };
}

interface StoryResponse {
  story?: {
    author_profile_id?: string;
  };
}

interface MessageListResponse {
  message_list?: {
    messages?: Array<{
      id?: string;
      sender_profile_id?: string;
    }>;
  };
}

function parseEvidenceChatId(evidenceJson: string): string | undefined {
  try {
    const parsed = JSON.parse(evidenceJson) as { chat_id?: string };
    const chatId = parsed.chat_id?.trim();
    return chatId || undefined;
  } catch {
    return undefined;
  }
}

async function resolveMessageAuthorAccount(
  messageId: string,
  evidenceJson: string,
): Promise<string | undefined> {
  const chatId = parseEvidenceChatId(evidenceJson);
  if (!chatId) {
    return undefined;
  }
  const search = new URLSearchParams({ chat_id: chatId, page_size: "100" });
  const data = await apiJson<MessageListResponse>(
    `/api/v1/messages?${search.toString()}`,
  );
  const message = data.message_list?.messages?.find((row) => row.id === messageId);
  if (!message?.sender_profile_id) {
    return undefined;
  }
  return resolveAccountIdForProfile(message.sender_profile_id);
}

export async function resolveTargetAccountId(
  report: Report,
): Promise<string | undefined> {
  switch (report.target_type) {
    case "user":
      return resolveAccountIdForProfile(report.target_id);
    case "space": {
      const data = await apiJson<SpaceResponse>(
        `/api/v1/spaces/${encodeURIComponent(report.target_id)}`,
      );
      const ownerProfileId = data.space?.owner_profile_id;
      if (!ownerProfileId) {
        return undefined;
      }
      return resolveAccountIdForProfile(ownerProfileId);
    }
    case "story": {
      const data = await apiJson<StoryResponse>(
        `/api/v1/stories/${encodeURIComponent(report.target_id)}`,
      );
      const authorProfileId = data.story?.author_profile_id;
      if (!authorProfileId) {
        return undefined;
      }
      return resolveAccountIdForProfile(authorProfileId);
    }
    case "message":
      return resolveMessageAuthorAccount(report.target_id, report.evidence_json);
    default:
      return undefined;
  }
}
