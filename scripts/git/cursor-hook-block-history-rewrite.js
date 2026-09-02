#!/usr/bin/env node
/**
 * beforeShellExecution: block VCS history rewrites and hook bypass.
 * Canonical copy lives in Voice repo: scripts/git/cursor-hook-block-history-rewrite.js
 *
 * Windows: Cursor/PowerShell may deliver stdin as UTF-8 BOM or UTF-16 LE.
 */
"use strict";

const { readFileSync } = require("fs");

function readStdinText() {
  let buf;
  try {
    buf = readFileSync(0);
  } catch {
    return "";
  }
  if (!buf || buf.length === 0) return "";

  if (buf.length >= 3 && buf[0] === 0xef && buf[1] === 0xbb && buf[2] === 0xbf) {
    return buf.slice(3).toString("utf8");
  }
  if (buf.length >= 2 && buf[0] === 0xff && buf[1] === 0xfe) {
    return buf.slice(2).toString("utf16le");
  }
  if (buf.length >= 2 && buf[0] === 0xfe && buf[1] === 0xff) {
    const swapped = Buffer.alloc(buf.length - 2);
    for (let i = 2; i + 1 < buf.length; i += 2) {
      swapped[i - 2] = buf[i + 1];
      swapped[i - 1] = buf[i];
    }
    return swapped.toString("utf16le");
  }
  if (buf.length >= 4 && buf[1] === 0x00 && buf[3] === 0x00) {
    return buf.toString("utf16le").replace(/^\uFEFF/, "");
  }

  return buf.toString("utf8").replace(/^\uFEFF/, "");
}

function parsePayload(raw) {
  const cleaned = String(raw || "").trim();
  if (!cleaned) {
    return { command: "" };
  }
  return JSON.parse(cleaned);
}

function deny(userMessage, agentMessage) {
  process.stdout.write(
    JSON.stringify({
      permission: "deny",
      user_message: userMessage,
      agent_message: agentMessage,
    })
  );
}

function allow() {
  process.stdout.write(JSON.stringify({ permission: "allow" }));
}

function hasGit(cmd) {
  return /\bgit(\.exe)?\b/i.test(cmd);
}

function isForcePush(cmd) {
  if (!/\bpush\b/i.test(cmd)) return false;

  if (
    /(^|[\s"'])(--force-with-lease|--force-if-includes|--force|-f)(?=([\s"'=]|$))/i.test(
      cmd
    )
  ) {
    return true;
  }

  if (/\bpush\b[\s\S]*\s\+[^\s]/i.test(cmd)) {
    return true;
  }

  return false;
}

function isRebase(cmd) {
  if (/\brebase\b/i.test(cmd)) return true;
  if (
    /\bpull\b/i.test(cmd) &&
    /(^|[\s"'])(--rebase|-r)(?=([\s"'=]|$))/i.test(cmd)
  ) {
    return true;
  }
  return false;
}

function isNoVerify(cmd) {
  if (!/(^|[\s"'])(--no-verify|-n)(?=([\s"'=]|$))/i.test(cmd)) {
    return false;
  }
  return /\b(commit|push)\b/i.test(cmd);
}

function isCommitAmend(cmd) {
  if (!/\bcommit\b/i.test(cmd)) return false;
  return /(^|[\s"'])(--amend|-amend)(?=([\s"'=]|$))/i.test(cmd);
}

function isResetHard(cmd) {
  if (!/\breset\b/i.test(cmd)) return false;
  return /(^|[\s"'])(--hard|-hard)(?=([\s"'=]|$))/i.test(cmd);
}

function isFilterBranch(cmd) {
  return /\b(filter-branch|filter-repo)\b/i.test(cmd);
}

function main() {
  let command = "";
  try {
    const payload = parsePayload(readStdinText());
    command = typeof payload.command === "string" ? payload.command : "";
  } catch {
    deny(
      "History-rewrite hook could not parse stdin; blocked for safety.",
      "beforeShellExecution hook failed to parse input; treat as deny and ask the user."
    );
    return;
  }

  if (!hasGit(command)) {
    allow();
    return;
  }

  if (isNoVerify(command)) {
    deny(
      "Blocked: --no-verify / -n on git commit or push is forbidden (fix files; do not bypass hooks).",
      "Do not use --no-verify or -n. Fix the hook failure (lint/format/tests), then create a new normal commit. History/hook bypass is blocked."
    );
    return;
  }

  if (isCommitAmend(command)) {
    deny(
      "Blocked: git commit --amend rewrites history.",
      "Do not amend. Create a new commit with fixes instead, or ask the user to amend manually."
    );
    return;
  }

  if (isResetHard(command)) {
    deny(
      "Blocked: git reset --hard is forbidden (destructive history/working-tree reset).",
      "Do not run reset --hard. Stash, revert, or ask the user before discarding work."
    );
    return;
  }

  if (isFilterBranch(command)) {
    deny(
      "Blocked: filter-branch / filter-repo rewrites history.",
      "Do not rewrite history with filter-branch or filter-repo. Ask the user if history rewrite is truly required."
    );
    return;
  }

  if (isForcePush(command)) {
    deny(
      "Blocked: force-push / history rewrite on remote is forbidden (any branch).",
      "Do not force-push. History rewrite is blocked by a user hook. Use a non-force push, open a new branch, or ask the user to run the force-push manually."
    );
    return;
  }

  if (isRebase(command)) {
    deny(
      "Blocked: rebase is forbidden (prefer merge; do not rewrite history).",
      "Do not rebase. History rewrite is blocked by a user hook. Merge origin/master (or the target branch) instead, or ask the user to rebase manually."
    );
    return;
  }

  allow();
}

try {
  main();
} catch {
  deny(
    "History-rewrite hook crashed; blocked for safety.",
    "beforeShellExecution hook error; treat as deny."
  );
}
