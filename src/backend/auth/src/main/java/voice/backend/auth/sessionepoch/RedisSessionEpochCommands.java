package voice.backend.auth.sessionepoch;

import java.time.Duration;

interface RedisSessionEpochCommands {
  long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout);

  long readRequiredPositive(String key, Duration timeout);
}
