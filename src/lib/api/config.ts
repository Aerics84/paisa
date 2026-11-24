import { request } from "./client";
import type { ConfigResponse } from "./types.gen";

export function getConfig(): Promise<ConfigResponse> {
  return request<ConfigResponse>("/api/config");
}

export function saveConfig(content: string): Promise<{ success: boolean; error?: string }> {
  return request<{ success: boolean; error?: string }>("/api/config", {
    method: "POST",
    body: content
  });
}
