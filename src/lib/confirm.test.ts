import { afterEach, describe, expect, test } from "bun:test";
import { get } from "svelte/store";
import { askConfirm, confirmRequest, resolveConfirm } from "./confirm";

afterEach(() => {
  // Drain anything left behind by a failing assertion so tests stay isolated.
  while (get(confirmRequest)) {
    resolveConfirm(false);
  }
});

describe("askConfirm", () => {
  test("opens a request and resolves true when confirmed", async () => {
    expect(get(confirmRequest)).toBeNull();

    const pending = askConfirm({
      title: "Delete template?",
      description: "This cannot be undone.",
      confirmLabel: "Delete",
      cancelLabel: "Cancel",
      variant: "danger"
    });

    const request = get(confirmRequest);
    expect(request?.title).toBe("Delete template?");
    expect(request?.description).toBe("This cannot be undone.");
    expect(request?.confirmLabel).toBe("Delete");
    expect(request?.cancelLabel).toBe("Cancel");
    expect(request?.variant).toBe("danger");

    resolveConfirm(true);

    await expect(pending).resolves.toBe(true);
    expect(get(confirmRequest)).toBeNull();
  });

  test("resolves false when the request is cancelled", async () => {
    const pending = askConfirm({ title: "Leave without saving?" });

    expect(get(confirmRequest)?.title).toBe("Leave without saving?");

    resolveConfirm(false);

    await expect(pending).resolves.toBe(false);
    expect(get(confirmRequest)).toBeNull();
  });

  test("serializes concurrent requests in FIFO order", async () => {
    const first = askConfirm({ title: "First" });
    const second = askConfirm({ title: "Second" });

    // The second request must wait until the first one has been answered.
    expect(get(confirmRequest)?.title).toBe("First");

    resolveConfirm(true);

    await expect(first).resolves.toBe(true);
    expect(get(confirmRequest)?.title).toBe("Second");

    resolveConfirm(false);

    await expect(second).resolves.toBe(false);
    expect(get(confirmRequest)).toBeNull();
  });
});
