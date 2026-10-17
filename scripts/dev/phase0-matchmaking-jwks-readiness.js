#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const https = require("node:https");

const endpoint = new URL("https://voice-matchmaking:8443/internal/v1/principal/jwks.json");
const ATTEMPT_TIMEOUT_MS = 2000;
const FAILURE_CATEGORIES = new Set([
  "ca_file_unavailable",
  "jwks_tls_verification_failed",
  "jwks_endpoint_unavailable",
  "jwks_http_response_invalid",
  "jwks_document_invalid",
  "jwks_response_too_large",
  "jwks_request_timeout",
  "jwks_transport_failed",
]);

function safeRequestFailure(error) {
  const code = error && typeof error.code === "string" ? error.code : "";
  if (["CERT_HAS_EXPIRED", "ERR_TLS_CERT_ALTNAME_INVALID", "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
       "SELF_SIGNED_CERT_IN_CHAIN", "DEPTH_ZERO_SELF_SIGNED_CERT"].includes(code)) {
    return "jwks_tls_verification_failed";
  }
  if (["ECONNREFUSED", "ECONNRESET", "EHOSTUNREACH", "ENETUNREACH", "ENOTFOUND", "EAI_AGAIN"].includes(code)) {
    return "jwks_endpoint_unavailable";
  }
  return "jwks_transport_failed";
}

function fetchPublicJwks(ca) {
  return new Promise((resolve, reject) => {
    let request;
    let settled = false;
    let deadline;
    const finish = (error) => {
      if (settled) return;
      settled = true;
      clearTimeout(deadline);
      if (error) reject(error);
      else resolve();
    };
    deadline = setTimeout(() => {
      finish(new Error("jwks_request_timeout"));
      if (request) request.destroy();
    }, ATTEMPT_TIMEOUT_MS);
    try {
      request = https.get({
        hostname: endpoint.hostname,
        port: endpoint.port,
        path: endpoint.pathname,
        ca,
        servername: endpoint.hostname,
      }, (response) => {
        if (response.statusCode !== 200 || response.headers.location ||
            !["application/jwk-set+json", "application/json"].some((contentType) =>
              String(response.headers["content-type"] || "").toLowerCase().startsWith(contentType))) {
          response.resume();
        finish(new Error("jwks_http_response_invalid"));
          return;
        }
        const chunks = [];
        let bytes = 0;
        response.on("data", (chunk) => {
          bytes += chunk.length;
          if (bytes > 65536) {
            finish(new Error("jwks_response_too_large"));
            request.destroy();
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
              finish(new Error("jwks_document_invalid"));
              return;
            }
            finish();
          } catch (_error) {
            finish(new Error("jwks_document_invalid"));
          }
        });
        response.on("error", finish);
      });
      request.on("error", (error) => finish(new Error(safeRequestFailure(error))));
    } catch (error) {
      finish(new Error(safeRequestFailure(error)));
    }
  });
}

async function main() {
  let ca;
  try {
    ca = fs.readFileSync("/run/phase0/ca.crt");
  } catch (_error) {
    emitFailure("ca_file_unavailable");
    return;
  }
  let lastFailure = "jwks_transport_failed";
  for (let attempt = 0; attempt < 30; attempt += 1) {
    try {
      await fetchPublicJwks(ca);
      return;
    } catch (error) {
      const category = error instanceof Error ? error.message : "";
      lastFailure = FAILURE_CATEGORIES.has(category) ? category : "jwks_transport_failed";
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
  }
  emitFailure(lastFailure);
}

function emitFailure(category) {
  const safe = FAILURE_CATEGORIES.has(category) ? category : "jwks_transport_failed";
  process.stderr.write(`phase0_matchmaking_jwks_readiness_failed category=${safe}\n`);
  process.exitCode = 1;
}

main().catch(() => emitFailure("jwks_transport_failed"));
