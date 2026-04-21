#!/usr/bin/env node
// CI-enforced parity check over frontend/messages/*.json.
// Fails (exit 1) if:
//   - any locale has a key the source (en) is missing
//   - the source has a key any target locale is missing
//   - en.json contains a "[FR] ..." placeholder value (wrong source of truth)
//   - any value is an empty string
//
// Keys starting with "$" (e.g. "$schema") are metadata and excluded from parity.

import { readFile, readdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const messagesDir = join(here, '..', 'messages');
const SOURCE_TAG = 'en';

const failures = [];
const fail = (msg) => failures.push(msg);

const files = (await readdir(messagesDir)).filter((f) => f.endsWith('.json'));
const catalogs = {};
for (const f of files) {
  const tag = f.replace(/\.json$/, '');
  const raw = await readFile(join(messagesDir, f), 'utf8');
  try {
    catalogs[tag] = JSON.parse(raw);
  } catch (e) {
    fail(`${f}: invalid JSON — ${e.message}`);
  }
}

if (!catalogs[SOURCE_TAG]) {
  fail(`source catalog messages/${SOURCE_TAG}.json is missing`);
}

const messageKeys = (obj) => Object.keys(obj).filter((k) => !k.startsWith('$'));

const sourceKeys = new Set(messageKeys(catalogs[SOURCE_TAG] ?? {}));

for (const [k, v] of Object.entries(catalogs[SOURCE_TAG] ?? {})) {
  if (k.startsWith('$')) continue;
  if (typeof v !== 'string') continue;
  if (v.startsWith('[FR]')) {
    fail(`${SOURCE_TAG}.json key "${k}" has a [FR] placeholder — source must be canonical EN`);
  }
  if (v === '') {
    fail(`${SOURCE_TAG}.json key "${k}" is an empty string`);
  }
}

for (const [tag, cat] of Object.entries(catalogs)) {
  if (tag === SOURCE_TAG) continue;
  const tagKeys = new Set(messageKeys(cat));

  for (const k of sourceKeys) {
    if (!tagKeys.has(k)) {
      fail(`${tag}.json: missing key "${k}" present in ${SOURCE_TAG}.json`);
    }
  }
  for (const k of tagKeys) {
    if (!sourceKeys.has(k)) {
      fail(`${tag}.json: extra key "${k}" not present in ${SOURCE_TAG}.json`);
    }
  }
  for (const [k, v] of Object.entries(cat)) {
    if (k.startsWith('$')) continue;
    if (v === '') fail(`${tag}.json key "${k}" is an empty string`);
  }
}

if (failures.length > 0) {
  console.error(`i18n:check FAILED (${failures.length} issues):`);
  for (const f of failures) console.error(`  - ${f}`);
  process.exit(1);
}

console.log(`i18n:check OK — ${sourceKeys.size} keys, ${Object.keys(catalogs).length} locales`);
