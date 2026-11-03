// Initial-load JS budget (FE-04, spec FR-C1/NFR3): the entry chunk plus the lazy route chunks for
// /login and / (dashboard), each with its static-import closure, must stay <= 300 KB gzip.
// Reads dist/.vite/manifest.json (vite build.manifest: true). Node stdlib only.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { gzipSync } from "node:zlib";

const BUDGET_BYTES = 300 * 1024;
const DIST = "dist";
// Manifest keys of the chunks loaded before / and /login can render.
const ROOTS = {
  entry: "index.html",
  "/login": "src/routes/_auth/login.tsx",
  "/": "src/features/dashboard/index.ts",
};

const manifest = JSON.parse(readFileSync(join(DIST, ".vite", "manifest.json"), "utf8"));

function closure(key, seen = new Set()) {
  const chunk = manifest[key];
  if (!chunk) throw new Error(`check-bundle: ${key} not in manifest (route moved?)`);
  if (seen.has(key)) return seen;
  seen.add(key);
  for (const dep of chunk.imports ?? []) closure(dep, seen);
  return seen;
}

const gzipSize = (key) => gzipSync(readFileSync(join(DIST, manifest[key].file))).length;
const kb = (bytes) => `${(bytes / 1024).toFixed(1)} KB`;

const all = new Set();
for (const [label, key] of Object.entries(ROOTS)) {
  const keys = closure(key);
  for (const k of keys) all.add(k);
  console.log(`${label.padEnd(7)} ${kb([...keys].reduce((sum, k) => sum + gzipSize(k), 0))} gzip`);
}

const total = [...all].reduce((sum, k) => sum + gzipSize(k), 0);
console.log(`initial JS (entry + /login + /): ${kb(total)} gzip, budget ${kb(BUDGET_BYTES)}`);
if (total > BUDGET_BYTES) {
  for (const k of [...all].sort((a, b) => gzipSize(b) - gzipSize(a))) {
    console.error(`  ${kb(gzipSize(k)).padStart(9)}  ${manifest[k].file}  (${k})`);
  }
  console.error("check-bundle: over budget");
  process.exit(1);
}
