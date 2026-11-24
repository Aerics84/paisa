import { requestApi, type RequestOptions } from "./client";
import type { Rule as WireImportRule } from "./import_rule.gen";
import type { DeleteRuleResponse, RulesResponse, SaveRuleResponse } from "./types.gen";

/**
 * Domain rule. The server assigns `id`, so the UI may hold an unsaved rule
 * without one while the wire type always includes it.
 */
export type ImportRule = Omit<WireImportRule, "id"> & { id?: number };

/** Payload accepted by POST /api/rules. */
export type ImportRuleInput = Partial<Omit<ImportRule, "id">> &
  Pick<ImportRule, "name" | "target_account">;

export type RulesResult = { rules: ImportRule[] };
export type SaveRuleResult = {
  saved?: boolean;
  rule?: ImportRule;
  message?: string;
  error?: string;
};

export function ruleFromWire(wire: WireImportRule): ImportRule {
  return { ...wire, id: wire.id === 0 ? undefined : wire.id };
}

export async function getRules(options?: RequestOptions): Promise<RulesResult> {
  const response = await requestApi<RulesResponse>("/api/rules", options);
  return { rules: (response.rules ?? []).map(ruleFromWire) };
}

export async function saveRule(
  rule: ImportRuleInput,
  options?: RequestOptions
): Promise<SaveRuleResult> {
  const response = await requestApi<SaveRuleResponse & { message?: string }>("/api/rules", {
    method: "POST",
    body: JSON.stringify(rule),
    ...options
  });
  return {
    saved: response.saved,
    rule: response.rule ? ruleFromWire(response.rule) : undefined,
    message: response.message
  };
}

export async function deleteRule(
  id: number,
  options?: RequestOptions
): Promise<DeleteRuleResponse> {
  return requestApi<DeleteRuleResponse>(`/api/rules/${id}`, {
    method: "DELETE",
    ...options
  });
}
