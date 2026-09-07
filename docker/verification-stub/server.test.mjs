import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";

const serverPath = fileURLToPath(new URL("./server.mjs", import.meta.url));

async function waitUntilReady(child) {
  let baseUrl = "";
  child.stdout.setEncoding("utf8");
  child.stdout.on("data", (chunk) => {
    const match = chunk.match(/verification-stub listening (\d+)/);
    if (match) baseUrl = `http://127.0.0.1:${match[1]}`;
  });
  for (let attempt = 0; attempt < 40; attempt += 1) {
    if (child.exitCode !== null) {
      throw new Error(`verification stub exited with ${child.exitCode}`);
    }
    try {
      const response = baseUrl && (await fetch(`${baseUrl}/health`));
      if (response && response.ok) return baseUrl;
    } catch {
      // Server startup may race this probe.
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error("verification stub did not become ready");
}

async function postMail(baseUrl, to, subject, text) {
  return fetch(`${baseUrl}/emails`, {
    method: "POST",
    headers: {
      authorization: "Bearer re_compose_key",
      "content-type": "application/json",
    },
    body: JSON.stringify({
      from: "Voice <noreply@voice.test>",
      to: [to],
      subject,
      text,
    }),
  });
}

test("accepts Resend mail and exposes the latest message for one recipient", async (t) => {
  const child = spawn(process.execPath, [serverPath], {
    env: { ...process.env, PORT: "0" },
    stdio: ["ignore", "pipe", "pipe"],
  });
  t.after(() => child.kill());
  const baseUrl = await waitUntilReady(child);

  const recipient = `verification-${Date.now()}@voice-qa.test`;
  const otherRecipient = `other-${Date.now()}@voice-qa.test`;

  assert.equal(
    (await postMail(baseUrl, recipient, "First verification", "Your Voice verification code is 111111")).status,
    200,
  );
  assert.equal(
    (await postMail(baseUrl, otherRecipient, "Other verification", "Your Voice verification code is 222222")).status,
    200,
  );
  assert.equal(
    (await postMail(baseUrl, recipient, "Latest verification", "Your Voice verification code is 654321")).status,
    200,
  );

  const latestResponse = await fetch(
    `${baseUrl}/emails/latest?to=${encodeURIComponent(recipient)}`,
  );
  assert.equal(latestResponse.status, 200);
  const latest = await latestResponse.json();
  assert.match(latest.id, /^re_/);
  assert.deepEqual(latest.to, [recipient]);
  assert.equal(latest.subject, "Latest verification");
  assert.equal(latest.text, "Your Voice verification code is 654321");
  assert.match(latest.text, /\b654321\b/);

  const otherResponse = await fetch(
    `${baseUrl}/emails/latest?to=${encodeURIComponent(otherRecipient)}`,
  );
  assert.equal(otherResponse.status, 200);
  const other = await otherResponse.json();
  assert.deepEqual(other.to, [otherRecipient]);
  assert.match(other.text, /\b222222\b/);

  const missingResponse = await fetch(
    `${baseUrl}/emails/latest?to=${encodeURIComponent("missing@voice-qa.test")}`,
  );
  assert.equal(missingResponse.status, 404);
});
