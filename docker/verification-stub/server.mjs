// Compose-only IdP/DNS fixture for verification.md VR-02/03 live tests.
// Not used in staging/prod. Helix partner + YPP allowed + seeded TXT.
import http from "node:http";
import { randomUUID } from "node:crypto";
import { URL } from "node:url";

const dns = new Map();

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
    send(res, 200, {
      items: [
        {
          id: "yt-compose",
          snippet: { title: "Voice YPP" },
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
  send(res, 404, { error: "not_found" });
});

server.listen(4180, "0.0.0.0");
