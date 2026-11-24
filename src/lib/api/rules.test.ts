import { afterEach, expect, test, mock } from "bun:test";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function fetchReturning(body: unknown, status = 200) {
  return mock(
    async () => new Response(JSON.stringify(body), { status })
  ) as unknown as typeof fetch;
}

test("getRules loads the rule list", async () => {
  globalThis.fetch = fetchReturning({
    rules: [
      {
        id: 3,
        name: "Coffee",
        priority: 100,
        enabled: true,
        payee_pattern: "Coffee",
        payee_match_type: "contains",
        memo_pattern: "",
        memo_match_type: "contains",
        tx_type: "any",
        target_account: "Expenses:Coffee",
        tags: [],
        flag_for_review: false
      }
    ]
  });

  const { getRules } = await import("./rules");
  const { rules } = await getRules({ background: true });

  expect(rules).toHaveLength(1);
  expect(rules[0].name).toBe("Coffee");
  expect(rules[0].target_account).toBe("Expenses:Coffee");
});

test("saveRule posts the rule and returns the saved rule", async () => {
  let captured: RequestInit | undefined;
  globalThis.fetch = mock(async (_input, init) => {
    captured = init;
    return new Response(
      JSON.stringify({
        saved: true,
        rule: {
          id: 4,
          name: "Groceries",
          priority: 100,
          enabled: true,
          payee_pattern: "Whole Foods",
          payee_match_type: "contains",
          memo_pattern: "",
          memo_match_type: "contains",
          tx_type: "any",
          target_account: "Expenses:Groceries",
          tags: [],
          flag_for_review: false
        }
      })
    );
  }) as unknown as typeof fetch;

  const { saveRule } = await import("./rules");
  const response = await saveRule(
    {
      name: "Groceries",
      priority: 100,
      enabled: true,
      payee_pattern: "Whole Foods",
      payee_match_type: "contains",
      target_account: "Expenses:Groceries"
    },
    { background: true }
  );

  expect(captured?.method).toBe("POST");
  expect(JSON.parse(String(captured?.body))).toMatchObject({
    name: "Groceries",
    payee_pattern: "Whole Foods",
    target_account: "Expenses:Groceries"
  });
  expect(response.saved).toBe(true);
  expect(response.rule?.id).toBe(4);
});

test("deleteRule calls the id route", async () => {
  let captured: { input: RequestInfo | URL; init?: RequestInit } | undefined;
  globalThis.fetch = mock(async (input, init) => {
    captured = { input, init };
    return new Response(JSON.stringify({ success: true }));
  }) as unknown as typeof fetch;

  const { deleteRule } = await import("./rules");
  const response = await deleteRule(7, { background: true });

  expect(captured?.input).toBe("/api/rules/7");
  expect(captured?.init?.method).toBe("DELETE");
  expect(response.success).toBe(true);
});

test("saveRule surfaces the server error message", async () => {
  globalThis.fetch = fetchReturning({ error: "rule name is required" }, 400);

  const { saveRule } = await import("./rules");
  await expect(
    saveRule({ name: "", target_account: "Expenses:Unknown" }, { background: true })
  ).rejects.toThrow("rule name is required");
});
