#!/usr/bin/env node
"use strict";
const { spawnSync } = require("child_process");
const path = require("path");

const hook = path.join(__dirname, "cursor-hook-block-history-rewrite.js");
const cases = [
  ["force-with-lease", "git push --force-with-lease origin HEAD", "deny"],
  ["force", "git push --force", "deny"],
  ["-f", "git push -f origin HEAD", "deny"],
  ["+refspec", "git push origin +feature/x", "deny"],
  ["rebase", "git rebase origin/master", "deny"],
  ["pull --rebase", "git pull --rebase origin master", "deny"],
  ["commit --no-verify", "git commit --no-verify -m x", "deny"],
  ["commit -n", "git commit -n -m x", "deny"],
  ["push --no-verify", "git push --no-verify origin HEAD", "deny"],
  ["commit --amend", "git commit --amend --no-edit", "deny"],
  ["reset --hard", "git reset --hard HEAD~1", "deny"],
  ["filter-branch", "git filter-branch --all", "deny"],
  ["filter-repo", "git filter-repo --path foo", "deny"],
  ["BOM force", "\uFEFF" + JSON.stringify({ command: "git push --force" }), "deny", true],
  ["normal push", "git push -u origin HEAD", "allow"],
  ["normal commit", 'git commit -m "fix lint"', "allow"],
  ["status", "git status", "allow"],
  ["merge", "git merge origin/master", "allow"],
  ["reset soft", "git reset --soft HEAD~1", "allow"],
];

let failed = 0;
for (const [name, cmdOrRaw, expect, raw] of cases) {
  const input = raw ? cmdOrRaw : JSON.stringify({ command: cmdOrRaw });
  const r = spawnSync(process.execPath, [hook], {
    input,
    encoding: "utf8",
  });
  let perm = "?";
  try {
    perm = JSON.parse(r.stdout || "{}").permission;
  } catch {
    perm = `parse-fail:${r.stdout}`;
  }
  const ok = perm === expect;
  if (!ok) failed += 1;
  console.log(`${ok ? "OK" : "FAIL"}: ${name} -> ${perm} (expect ${expect})`);
}
process.exit(failed ? 1 : 0);
