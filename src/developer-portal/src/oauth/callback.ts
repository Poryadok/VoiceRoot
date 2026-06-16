export type CallbackParams = {
  code: string | null;
  state: string | null;
  error: string | null;
};

export function parseCallbackSearch(search: string): CallbackParams {
  const params = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search);
  return {
    code: params.get('code'),
    state: params.get('state'),
    error: params.get('error'),
  };
}

export function callbackRedirectUri(origin: string): string {
  return `${origin.replace(/\/$/, '')}/callback`;
}
