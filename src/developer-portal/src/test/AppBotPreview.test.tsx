import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { App } from '../App';

const savedBot = {
  id: 'a', name: 'Saved A', description: 'Published description', slug: 'alpha',
  scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
  owner_account_id: 'private-owner', webhook_url: 'https://private.example/hook',
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

function deferred() {
  let resolve!: (value: Response) => void;
  const promise = new Promise<Response>((done) => { resolve = done; });
  return { promise, resolve };
}

function setup(options: {
  bot?: Record<string, unknown>;
  secondDetail?: Promise<Response>;
  commandsFail?: boolean;
  detailReject?: boolean;
  commandsReject?: boolean;
  afterManifestDetail?: Promise<Response>;
} = {}) {
  let applied = false;
  vi.stubGlobal('fetch', vi.fn(async (input: unknown, init?: RequestInit) => {
    const path = new URL(String(input)).pathname;
    if (path === '/api/v1/bots') {
      return json({ bot_list: { bots: [{ id: 'a', name: 'A' }, { id: 'b', name: 'B' }] } });
    }
    if (path === '/api/v1/bots/a') {
      if (options.detailReject) throw new Error('network unavailable');
      if (applied && options.afterManifestDetail) return options.afterManifestDetail;
      return json({ bot: options.bot ?? savedBot });
    }
    if (path === '/api/v1/bots/b') {
      return options.secondDetail ?? json({ bot: { ...savedBot, id: 'b', name: 'Saved B', slug: 'beta' } });
    }
    if (path.endsWith('/commands')) {
      if (options.commandsReject) throw new Error('network unavailable');
      return options.commandsFail
        ? json({ error: 'unavailable' }, 503)
        : json({ command_list: { commands_json: applied
          ? '[{"name":"updated","description":"Updated command"}]'
          : '[{"name":"ping","description":"Ping command"}]' } });
    }
    if (path.endsWith('/manifest')) {
      if (init?.method === 'POST') applied = true;
      return json({ manifest_yaml: '' });
    }
    throw new Error(`Unexpected request: ${path}`);
  }));
}

describe('saved public bot preview', () => {
  beforeEach(() => {
    sessionStorage.setItem('voice_access_token', 'test-jwt');
    window.history.replaceState({}, '', '/');
  });

  afterEach(() => {
    cleanup();
    sessionStorage.clear();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('shows the server slug and saved public fields without unsaved or private data', async () => {
    setup();
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    const scope = within(preview);
    expect(scope.getByText('Saved A')).toBeInTheDocument();
    expect(scope.getByText('Published description')).toBeInTheDocument();
    expect(scope.getByText('Send messages in allowed text chats')).toBeInTheDocument();
    expect(await scope.findByText('/ping')).toBeInTheDocument();
    expect(scope.getByRole('link', { name: 'Open public bot page' })).toHaveAttribute('href', 'https://voice.gg/bots/alpha');
    expect(screen.getByLabelText('Public bot slug')).toHaveAttribute('readonly');
    expect(screen.getByLabelText('Public bot slug')).toHaveValue('alpha');
    fireEvent.change(within(screen.getByTestId('bot-lifecycle')).getByLabelText('Bot name'), {
      target: { value: 'Unsaved name' },
    });
    expect(scope.queryByText('Unsaved name')).not.toBeInTheDocument();
    expect(preview).not.toHaveTextContent('private-owner');
    expect(preview).not.toHaveTextContent('private.example');
  });

  it('does not invent a public link when a legacy bot has no slug', async () => {
    setup({ bot: { ...savedBot, slug: undefined } });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(within(preview).queryByRole('link')).not.toBeInTheDocument();
    expect(within(preview).getByText('Public slug unavailable')).toBeInTheDocument();
  });

  it('encodes the returned slug as one path component', async () => {
    setup({ bot: { ...savedBot, slug: 'alpha/?#' } });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(within(preview).getByRole('link')).toHaveAttribute('href', 'https://voice.gg/bots/alpha%2F%3F%23');
    expect(within(preview).getByRole('link')).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('clears the preview during selection and ignores a late previous selection response', async () => {
    const pending = deferred();
    setup({ secondDetail: pending.promise });
    render(<App />);
    await screen.findByTestId('public-bot-preview');
    fireEvent.click(screen.getByRole('button', { name: 'B' }));
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'A' }));
    await screen.findByTestId('public-bot-preview');
    await act(async () => {
      pending.resolve(json({ bot: { ...savedBot, id: 'b', name: 'Saved B', slug: 'beta' } }));
      await pending.promise;
    });
    expect(within(screen.getByTestId('public-bot-preview')).getByText('Saved A')).toBeInTheDocument();
    expect(within(screen.getByTestId('public-bot-preview')).queryByText('Saved B')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
  });

  it('does not accept a detail response for a different bot', async () => {
    setup({ bot: { ...savedBot, id: 'wrong' } });
    render(<App />);
    await screen.findByText('Load bot failed: mismatched bot response');
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save bot changes' })).toBeDisabled();
  });

  it('distinguishes unavailable commands from an empty command list', async () => {
    setup({ commandsFail: true });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(await within(preview).findByText('Command preview unavailable')).toBeInTheDocument();
  });

  it('renders unknown scope strings literally without inheriting object properties or HTML', async () => {
    setup({ bot: { ...savedBot, scopes_json: '["__proto__","<img src=x>"]' } });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(within(preview).getByText('__proto__')).toBeInTheDocument();
    expect(within(preview).getByText('<img src=x>')).toBeInTheDocument();
    expect(preview.querySelector('img')).toBeNull();
  });

  it('shows malformed scope data as unavailable rather than no permissions', async () => {
    setup({ bot: { ...savedBot, scopes_json: '{"not":"an array"}' } });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(within(preview).getByText('Requested scopes unavailable')).toBeInTheDocument();
    expect(within(preview).queryByText('No requested scopes')).not.toBeInTheDocument();
  });

  it('handles a detail network failure without displaying stale public data', async () => {
    setup({ detailReject: true });
    render(<App />);
    await screen.findByText('Load bot failed: unavailable response');
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save bot changes' })).toBeDisabled();
  });

  it('handles a command network failure as unavailable rather than endless loading', async () => {
    setup({ commandsReject: true });
    render(<App />);
    const preview = await screen.findByTestId('public-bot-preview');
    expect(await within(preview).findByText('Command preview unavailable')).toBeInTheDocument();
    expect(within(preview).queryByText('Loading command preview…')).not.toBeInTheDocument();
  });

  it('invalidates saved public data after applying a manifest and reloads detail with commands', async () => {
    const pending = deferred();
    setup({ afterManifestDetail: pending.promise });
    render(<App />);
    await within(await screen.findByTestId('public-bot-preview')).findByText('/ping');
    fireEvent.click(screen.getByRole('button', { name: 'Apply to bot' }));
    await screen.findByText('Manifest applied');
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save bot changes' })).toBeDisabled();
    await act(async () => {
      pending.resolve(json({ bot: { ...savedBot, name: 'Manifest name', description: 'Manifest description', scopes_json: '[]' } }));
      await pending.promise;
    });
    const preview = within(await screen.findByTestId('public-bot-preview'));
    expect(preview.getByText('Manifest name')).toBeInTheDocument();
    expect(preview.getByText('Manifest description')).toBeInTheDocument();
    expect(preview.getByText('No requested scopes')).toBeInTheDocument();
    expect(await preview.findByText('/updated')).toBeInTheDocument();
    expect(preview.queryByText('Saved A')).not.toBeInTheDocument();
    expect(preview.queryByText('Send messages in allowed text chats')).not.toBeInTheDocument();
  });

  it.each(['selection', 'logout'])('ignores a late manifest detail refresh after %s', async (change) => {
    const pending = deferred();
    setup({ afterManifestDetail: pending.promise });
    render(<App />);
    await within(await screen.findByTestId('public-bot-preview')).findByText('/ping');
    fireEvent.click(screen.getByRole('button', { name: 'Apply to bot' }));
    await screen.findByText('Manifest applied');
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    if (change === 'selection') {
      fireEvent.click(screen.getByRole('button', { name: 'B' }));
      await within(await screen.findByTestId('public-bot-preview')).findByText('Saved B');
    } else {
      fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    }
    await act(async () => {
      pending.resolve(json({ bot: { ...savedBot, name: 'Late manifest A' } }));
      await pending.promise;
    });
    expect(screen.queryByText('Late manifest A')).not.toBeInTheDocument();
    if (change === 'selection') {
      expect(within(screen.getByTestId('public-bot-preview')).getByText('Saved B')).toBeInTheDocument();
    } else {
      expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    }
  });

  it('keeps stale public data hidden when post-manifest detail refresh fails', async () => {
    const pending = deferred();
    setup({ afterManifestDetail: pending.promise });
    render(<App />);
    await within(await screen.findByTestId('public-bot-preview')).findByText('/ping');
    fireEvent.click(screen.getByRole('button', { name: 'Apply to bot' }));
    await screen.findByText('Manifest applied');
    await act(async () => {
      pending.resolve(json({ error: 'unavailable' }, 503));
      await pending.promise;
    });
    await screen.findByText(/Load bot failed:/);
    expect(screen.queryByTestId('public-bot-preview')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save bot changes' })).toBeDisabled();
  });
});
