#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const https = require("node:https");

const endpoint = new URL("https://phase0-jwks:8443/matchmaking/jwks.json");
const ca = fs.readFileSync("/run/phase0/ca.crt");

function fetchPublicJwks() {
  return new Promise((resolve, reject) => {
    const request = https.get({
      hostname: endpoint.hostname,
      port: endpoint.port,
      path: endpoint.pathname,
      ca,
      servername: endpoint.hostname,
      timeout: 2000,
    }, (response) => {
      if (response.statusCode !== 200 || response.headers.location ||
          !String(response.headers["content-type"] || "").toLowerCase().startsWith("application/json")) {
        response.resume();
        reject(new Error("jwks_response_invalid"));
        return;
      }
      const chunks = [];
      let bytes = 0;
      response.on("data", (chunk) => {
        bytes += chunk.length;
        if (bytes > 65536) {
          request.destroy(new Error("jwks_response_too_large"));
          return;
        }
        chunks.push(chunk);
      });
      response.on("end", () => {
        try {
          const document = JSON.parse(Buffer.concat(chunks).toString("utf8"));
          const keys = document.keys;
          const valid = Array.isArray(keys) && keys.length > 0 && keys.every((key) =>
            key && key.kty === "RSA" && key.use === "sig" && key.alg === "RS256" &&
            typeof key.kid === "string" && key.kid.length > 0 &&
            typeof key.n === "string" && typeof key.e === "string" &&
            !["d", "p", "q", "dp", "dq", "qi", "oth"].some((privateField) =>
              Object.hasOwn(key, privateField)));
          if (!valid) {
            reject(new Error("jwks_document_invalid"));
            return;
          }
          resolve();
        } catch (_error) {
          reject(new Error("jwks_document_invalid"));
        }
      });
      response.on("error", reject);
    });
    request.on("timeout", () => request.destroy(new Error("jwks_request_timeout")));
    request.on("error", reject);
  });
}

async function main() {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    try {
      await fetchPublicJwks();
      return;
    } catch (_error) {
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
  }
  process.stderr.write("phase0_matchmaking_jwks_readiness_failed\n");
  process.exitCode = 1;
}

main().catch(() => {
  process.stderr.write("phase0_matchmaking_jwks_readiness_failed\n");
  process.exitCode = 1;
});
