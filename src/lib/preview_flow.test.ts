import { describe, expect, test } from "bun:test";
import { createFlowRenderController, flowSignature, flowState } from "./preview_flow";
import type { Graph } from "./utils";

const empty: Graph = { nodes: [], links: [] };
const ready: Graph = {
  nodes: [
    { id: 1, name: "Income" },
    { id: 2, name: "Assets:Checking" }
  ],
  links: [{ source: 1, target: 2, value: 1000 }]
};

describe("preview flow state", () => {
  test("requires both nodes and links", () => {
    expect(flowState(null)).toBe("empty");
    expect(flowState({ nodes: ready.nodes, links: [] })).toBe("empty");
    expect(flowState({ nodes: [], links: ready.links })).toBe("empty");
    expect(flowState(ready)).toBe("ready");
  });
});

describe("preview flow signature", () => {
  test("changes when the graph changes for the same month", () => {
    const first = flowSignature({ month: "2026-10", flow_graph: ready });
    const changed = flowSignature({
      month: "2026-10",
      flow_graph: { ...ready, links: [{ source: 1, target: 2, value: 2000 }] }
    });
    expect(first).not.toBe(changed);
  });

  test("is stable for the same graph", () => {
    expect(flowSignature({ month: "2026-10", flow_graph: ready })).toBe(
      flowSignature({ month: "2026-10", flow_graph: ready })
    );
  });

  test("represents an empty review distinctly from no review", () => {
    expect(flowSignature({ month: "2026-10", flow_graph: empty })).not.toBe("");
    expect(flowSignature(null)).toBe("");
  });

  test("normalizes node order and already-resolved D3 link endpoints", () => {
    const reordered: Graph = {
      nodes: [ready.nodes[1], ready.nodes[0]],
      links: [
        {
          source: ready.nodes[0] as any,
          target: ready.nodes[1] as any,
          value: 1000
        }
      ]
    };
    expect(flowSignature({ month: "2026-10", flow_graph: reordered })).toBe(
      flowSignature({ month: "2026-10", flow_graph: ready })
    );
  });
});

describe("preview flow render controller", () => {
  test("renders ready, replaces changed, clears empty and renders again", () => {
    const calls: string[] = [];
    const controller = createFlowRenderController({
      render: (graph) => calls.push(`render:${graph.links[0]?.value ?? 0}`),
      clear: () => calls.push("clear")
    });

    expect(controller.update(ready, "A", "ready")).toBe(true);
    expect(controller.update(ready, "A", "ready")).toBe(false);
    expect(
      controller.update({ ...ready, links: [{ source: 1, target: 2, value: 2000 }] }, "B", "ready")
    ).toBe(true);
    expect(controller.update(empty, "C", "empty")).toBe(true);
    expect(controller.update(ready, "D", "ready")).toBe(true);
    expect(calls).toEqual(["render:1000", "render:2000", "clear", "render:1000"]);
  });

  test("does not mark a failed render as complete and allows a retry", () => {
    let shouldFail = true;
    const controller = createFlowRenderController({
      render: () => {
        if (shouldFail) throw new Error("canvas missing");
      },
      clear: () => undefined
    });

    expect(() => controller.update(ready, "A", "ready")).toThrow("canvas missing");
    expect(controller.renderedSignature()).toBe("");
    shouldFail = false;
    expect(controller.update(ready, "A", "ready")).toBe(true);
    expect(controller.renderedSignature()).toBe("A");
  });

  test("can be reset when the canvas is recreated", () => {
    let renders = 0;
    const controller = createFlowRenderController({
      render: () => {
        renders++;
      },
      clear: () => undefined
    });

    expect(controller.update(ready, "A", "ready")).toBe(true);
    expect(controller.update(ready, "A", "ready")).toBe(false);
    controller.reset();
    expect(controller.update(ready, "A", "ready")).toBe(true);
    expect(renders).toBe(2);
  });
});
