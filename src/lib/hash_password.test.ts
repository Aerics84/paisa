import { afterEach, expect, test } from "bun:test";
import { loading } from "../store";
import { hashPassword } from "./utils";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  loading.set(false);
});

function mockResponse(status: number, body: unknown) {
  globalThis.fetch = (() =>
    Promise.resolve({
      status,
      text: async () => JSON.stringify(body)
    })) as unknown as typeof fetch;
}

test("hashPassword rejects with the server message on HTTP 400", async () => {
  mockResponse(400, { error: "password must be at least 8 characters" });

  await expect(hashPassword("short")).rejects.toThrow(
    "password must be at least 8 characters"
  );
});

test("hashPassword rejects with the server message on HTTP 500", async () => {
  mockResponse(500, { error: "could not hash password" });

  await expect(hashPassword("longenough")).rejects.toThrow(
    "could not hash password"
  );
});

test("hashPassword rejects a successful response without a valid hash", async () => {
  mockResponse(200, {});

  await expect(hashPassword("longenough")).rejects.toThrow(
    "Could not hash password"
  );
});

test("hashPassword returns a valid argon2id hash", async () => {
  const hash = "argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA";
  mockResponse(200, { hash });

  await expect(hashPassword("longenough")).resolves.toBe(hash);
});
