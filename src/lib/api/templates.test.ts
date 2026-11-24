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

test("getTemplates loads the template list", async () => {
  globalThis.fetch = fetchReturning({
    templates: [
      { id: "builtin:ICICI", name: "ICICI", content: "{{date}}", template_type: "builtin" }
    ]
  });

  const { getTemplates } = await import("./templates");
  const { templates } = await getTemplates();

  expect(templates).toHaveLength(1);
  expect(templates[0].name).toBe("ICICI");
  expect(templates[0].template_type).toBe("builtin");
});

test("upsertTemplate posts name and content", async () => {
  let captured: RequestInit | undefined;
  globalThis.fetch = mock(async (_input, init) => {
    captured = init;
    return new Response(
      JSON.stringify({
        saved: true,
        template: {
          id: "custom:Mine",
          name: "Mine",
          content: "{{payee}}",
          template_type: "custom"
        }
      })
    );
  }) as unknown as typeof fetch;

  const { upsertTemplate } = await import("./templates");
  const response = await upsertTemplate(
    { name: "Mine", content: "{{payee}}" },
    { background: true }
  );

  expect(captured?.method).toBe("POST");
  expect(JSON.parse(String(captured?.body))).toEqual({
    name: "Mine",
    content: "{{payee}}"
  });
  expect(response.saved).toBe(true);
  expect(response.template?.id).toBe("custom:Mine");
});

test("deleteTemplate posts the template name", async () => {
  let captured: RequestInit | undefined;
  globalThis.fetch = mock(async (_input, init) => {
    captured = init;
    return new Response(JSON.stringify({ success: true }));
  }) as unknown as typeof fetch;

  const { deleteTemplate } = await import("./templates");
  const response = await deleteTemplate("Mine", { background: true });

  expect(captured?.method).toBe("POST");
  expect(JSON.parse(String(captured?.body))).toEqual({ name: "Mine" });
  expect(response.success).toBe(true);
});
