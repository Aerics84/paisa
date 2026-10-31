import type { Graph } from "./utils";
import type { MonthlyReviewResponse } from "./monthly_review";

export type FlowState = "empty" | "ready";

type NodeReference = number | { id: number };

function nodeId(value: NodeReference): number {
  return typeof value === "number" ? value : value.id;
}

export function flowState(graph: Graph | null | undefined): FlowState {
  return graph &&
    Array.isArray(graph.nodes) &&
    graph.nodes.length > 0 &&
    Array.isArray(graph.links) &&
    graph.links.length > 0
    ? "ready"
    : "empty";
}

export function flowSignature(
  review: Pick<MonthlyReviewResponse, "month" | "flow_graph"> | null
): string {
  if (!review) return "";
  const nodes = (review.flow_graph?.nodes ?? [])
    .map((node) => [node.id, node.name])
    .sort((a, b) => String(a[0]).localeCompare(String(b[0])));
  const links = (review.flow_graph?.links ?? [])
    .map((link) => [
      nodeId(link.source as NodeReference),
      nodeId(link.target as NodeReference),
      link.value
    ])
    .sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
  return JSON.stringify({ month: review.month, nodes, links });
}

export interface FlowRenderController {
  update(graph: Graph, signature: string, state: FlowState): boolean;
  renderedSignature(): string;
  reset(): void;
}

export function createFlowRenderController(handlers: {
  render: (graph: Graph) => void;
  clear: () => void;
}): FlowRenderController {
  let last = "";
  return {
    update(graph, signature, state) {
      if (signature === last) return false;
      if (state === "empty") {
        handlers.clear();
        last = signature;
        return true;
      }
      handlers.render(graph);
      last = signature;
      return true;
    },
    renderedSignature() {
      return last;
    },
    reset() {
      last = "";
    }
  };
}
