import { afterEach, expect, test } from "bun:test";
import { get } from "svelte/store";
import { beginPasswordHash, endPasswordHash, pendingPasswordHashes } from "./password_hash";

afterEach(() => {
  pendingPasswordHashes.set(0);
});

test("tracks in-flight password hashes", () => {
  expect(get(pendingPasswordHashes)).toBe(0);

  beginPasswordHash();
  beginPasswordHash();
  expect(get(pendingPasswordHashes)).toBe(2);

  endPasswordHash();
  expect(get(pendingPasswordHashes)).toBe(1);

  endPasswordHash();
  expect(get(pendingPasswordHashes)).toBe(0);
});

test("does not go negative when a hash finishes twice", () => {
  endPasswordHash();
  expect(get(pendingPasswordHashes)).toBe(0);
});
