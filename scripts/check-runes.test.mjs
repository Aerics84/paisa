import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";

import { countLegacyUsage } from "./check-runes.mjs";

function tempDir(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "paisa-runes-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function writeFixture(t, name, content) {
  const file = path.join(tempDir(t), name);
  fs.writeFileSync(file, content);
  return file;
}

test("counts every legacy pattern", (t) => {
  const file = writeFixture(
    t,
    "Widget.svelte",
    [
      "<script>",
      "  export let open = false;",
      "  $: doubled = open ? 2 : 0;",
      "  const dispatch = createEventDispatcher();",
      "</script>",
      "<button on:click={() => (open = !open)}>Toggle</button>"
    ].join("\n")
  );

  const { counts } = countLegacyUsage([file]);
  assert.deepEqual(counts, {
    "export let": 1,
    "$:": 1,
    "on:click": 1,
    createEventDispatcher: 1
  });
});

test("counts $: only when it starts a line", (t) => {
  const file = writeFixture(
    t,
    "Inline.svelte",
    ["<script>", "  let a = 1; $: ignored = a * 2;", "  $: counted = a + 1;", "</script>"].join(
      "\n"
    )
  );

  const { counts } = countLegacyUsage([file]);
  assert.equal(counts["$:"], 1);
});

test("handles CRLF line endings", (t) => {
  const file = writeFixture(
    t,
    "Windows.svelte",
    ["<script>", "  export let x = 0;", "  $: y = x + 1;", "</script>"].join("\r\n")
  );

  const { counts } = countLegacyUsage([file]);
  assert.equal(counts["export let"], 1);
  assert.equal(counts["$:"], 1);
});

test("counts commented mentions, as documented for textual counting", (t) => {
  const file = writeFixture(
    t,
    "Commented.svelte",
    [
      "<!-- on:click in a comment -->",
      "<script>",
      "  // createEventDispatcher here",
      "</script>"
    ].join("\n")
  );

  const { counts } = countLegacyUsage([file]);
  assert.equal(counts["on:click"], 1);
  assert.equal(counts.createEventDispatcher, 1);
});
