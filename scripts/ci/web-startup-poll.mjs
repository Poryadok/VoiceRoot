import { setTimeout as delay } from 'node:timers/promises';

const isNavigationInterruption = error => error.code === -32000 && error.message === 'Inspected target navigated or closed';

export async function clickRetry(command, point) {
  await command('Input.dispatchMouseEvent', { type: 'mousePressed', ...point, button: 'left', clickCount: 1 });
  try {
    await command('Input.dispatchMouseEvent', { type: 'mouseReleased', ...point, button: 'left', clickCount: 1 });
  } catch (error) {
    if (!isNavigationInterruption(error)) throw error;
  }
}

export async function pollAfterNavigation(check, { interval = 100, timeout = 60000 } = {}) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try {
      const value = await check();
      if (value) return value;
    } catch (error) {
      if (!isNavigationInterruption(error)) throw error;
    }
    await delay(interval);
  }
  throw new Error('Timed out: retry reload reaches rendered app');
}
