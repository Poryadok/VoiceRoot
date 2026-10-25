// Compose-only IdP/DNS/email fixture for live tests. Not used in staging/prod.
// Helix partner + YPP allowed + seeded TXT + recipient-scoped Resend mail capture.
import http from "node:http";
import { randomUUID } from "node:crypto";
import { URL } from "node:url";

const dns = new Map();
const emails = new Map();
const mailModes = new Map();
const submissions = new Map();
const inbox = new Map();
const attempts = new Map();
const modes = new Set(["ACCEPT_THEN_DELIVER", "REJECT_AT_SUBMISSION", "ACCEPT_THEN_BOUNCE"]);
const normalizeRecipient = (value) => String(value || "").trim().toLowerCase();

function send(res, code, body, type = "application/json") {
  const data = typeof body === "string" ? body : JSON.stringify(body);
  res.writeHead(code, {
    "content-type": type,
    "content-length": Buffer.byteLength(data),
  });
  res.end(data);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let raw = "";
    req.on("data", (chunk) => {
      raw += chunk;
      if (raw.length > 1 << 20) {
        reject(new Error("body too large"));
      }
    });
    req.on("end", () => resolve(raw));
    req.on("error", reject);
  });
}

const server = http.createServer(async (req, res) => {
  const u = new URL(req.url || "/", "http://verification-stub");
  if (req.method === "GET" && (u.pathname === "/health" || u.pathname === "/")) {
    send(res, 200, "ok", "text/plain");
    return;
  }
  // These controls exist only in this disposable fixture, never in Auth.
  if (req.method === "POST" && ["/test/mail/mode", "/test/mail/settle"].includes(u.pathname)) {
    let body;
    try { body = JSON.parse((await readBody(req)) || "{}"); }
    catch { send(res, 400, { error: "invalid_json" }); return; }
    if (u.pathname === "/test/mail/mode") {
      const to = normalizeRecipient(body.to);
      if (!to || !modes.has(body.mode)) { send(res, 400, { error: "invalid_mail_mode" }); return; }
      mailModes.set(to, body.mode);
      send(res, 200, { to, mode: body.mode });
    } else {
      const message = submissions.get(body.id);
      if (!message) { send(res, 404, { error: "email_not_found" }); return; }
      if (message.status === "accepted") {
        message.status = message.outcome;
        if (message.status === "delivered") {
          for (const to of message.to) inbox.set(normalizeRecipient(to), message);
        }
      }
      send(res, 200, { id: message.id, status: message.status });
    }
    return;
  }
  if (req.method === "GET" && u.pathname.startsWith("/test/mail/")) {
    const to = normalizeRecipient(u.searchParams.get("to"));
    if (u.pathname === "/test/mail/attempts") {
      send(res, 200, { to, count: attempts.get(to) || 0 }); return;
    }
    const message = u.pathname === "/test/mail/inbox" ? inbox.get(to)
      : u.pathname === "/test/mail/status" ? submissions.get(u.searchParams.get("id")) : null;
    if (!message) { send(res, 404, { error: "email_not_found" }); return; }
    send(res, 200, message);
    return;
  }
  if (req.method === "GET" && u.pathname === "/helix/users") {
    // Unique external_id per OAuth code so compose live tests can link many accounts
    // (auth_db.linked_identities UNIQUE(platform, external_id)).
    const authHeader = req.headers.authorization || "";
    const tokenSuffix = authHeader.slice(-12) || "default";
    send(res, 200, {
      data: [
        {
          id: `tw-compose-${tokenSuffix}`,
          login: `voicepartner-${tokenSuffix}`,
          broadcaster_type: "partner",
        },
      ],
    });
    return;
  }
  if (req.method === "GET" && u.pathname === "/youtube/v3/channels") {
    const authHeader = req.headers.authorization || "";
    const tokenSuffix = authHeader.slice(-12) || "default";
    send(res, 200, {
      items: [
        {
          id: `yt-compose-${tokenSuffix}`,
          snippet: { title: `Voice YPP ${tokenSuffix}` },
          status: { longUploadsStatus: "allowed" },
        },
      ],
    });
    return;
  }
  if (req.method === "GET" && u.pathname === "/dns-txt") {
    const domain = (u.searchParams.get("domain") || "").trim().toLowerCase();
    send(res, 200, { records: dns.get(domain) || [] });
    return;
  }
  if (req.method === "GET" && u.pathname === "/emails/latest") {
    const recipient = (u.searchParams.get("to") || "").trim().toLowerCase();
    if (!recipient) {
      send(res, 400, { error: "recipient_required" });
      return;
    }
    const message = emails.get(recipient);
    if (!message) {
      send(res, 404, { error: "email_not_found" });
      return;
    }
    send(res, 200, message);
    return;
  }
  if (req.method === "POST" && u.pathname === "/oauth2/token") {
    const suffix = randomUUID().replace(/-/g, "").slice(0, 12);
    send(res, 200, {
      access_token: `stub-access-${suffix}`,
      token_type: "bearer",
    });
    return;
  }
  if (req.method === "PUT" && u.pathname === "/dns-txt") {
    let body;
    try {
      body = JSON.parse((await readBody(req)) || "{}");
    } catch {
      send(res, 400, { error: "invalid_json" });
      return;
    }
    const domain = String(body.domain || "")
      .trim()
      .toLowerCase();
    let records = body.records;
    if (!records && body.txt) {
      records = [body.txt];
    }
    if (!domain) {
      send(res, 400, { error: "domain_required" });
      return;
    }
    const list = Array.isArray(records) ? records.map(String) : [];
    dns.set(domain, list);
    send(res, 200, { domain, records: list });
    return;
  }
  if (req.method === "POST" && u.pathname === "/emails") {
    let body;
    try {
      body = JSON.parse((await readBody(req)) || "{}");
    } catch {
      send(res, 400, { error: "invalid_json" });
      return;
    }
    const recipients = Array.isArray(body.to)
      ? body.to.map((value) => String(value).trim()).filter(Boolean)
      : [];
    if (recipients.length === 0) {
      send(res, 400, { error: "recipient_required" });
      return;
    }
    // Controlled acceptance cases use one recipient so a single immutable ID
    // has one unambiguous terminal outcome. Legacy capture remains compatible.
    if (recipients.length > 1 && recipients.some((to) => mailModes.has(normalizeRecipient(to)))) {
      send(res, 400, { error: "controlled_mail_requires_single_recipient" }); return;
    }
    for (const to of recipients) {
      const key = normalizeRecipient(to);
      attempts.set(key, (attempts.get(key) || 0) + 1);
    }
    const mode = mailModes.get(normalizeRecipient(recipients[0])) || "ACCEPT_THEN_DELIVER";
    if (mode === "REJECT_AT_SUBMISSION") {
      send(res, 422, { error: "fixture_submission_rejected" }); return;
    }
    const message = {
      id: `re_${randomUUID().replace(/-/g, "")}`,
      to: recipients,
      subject: String(body.subject || ""),
      text: String(body.text || ""),
      status: "accepted",
      outcome: mode === "ACCEPT_THEN_BOUNCE" ? "bounced" : "delivered",
    };
    submissions.set(message.id, message);
    for (const recipient of recipients) {
      emails.set(recipient.toLowerCase(), message);
    }
    send(res, 200, { id: message.id });
    return;
  }
  send(res, 404, { error: "not_found" });
});

const port = Number.parseInt(process.env.PORT || "4180", 10);
server.listen(port, "0.0.0.0", () => {
  console.log(`verification-stub listening ${server.address().port}`);
});
