import { requestApi, type RequestOptions } from "./client";
import type { Template as WireImportTemplate } from "./template.gen";
import type {
  DeleteTemplateResponse,
  TemplatesResponse,
  UpsertTemplateResponse
} from "./types.gen";

/** Import template wire type from internal/model/template.Template. */
export type ImportTemplate = WireImportTemplate & { [key: string]: unknown };

/** Payload accepted by POST /api/templates/upsert. */
export type ImportTemplateInput = Pick<ImportTemplate, "name" | "content">;

export type TemplatesResult = { templates: ImportTemplate[] };
export type UpsertTemplateResult = {
  saved?: boolean;
  template?: ImportTemplate;
  message?: string;
};
export type DeleteTemplateResult = { success?: boolean; message?: string };

export async function getTemplates(options?: RequestOptions): Promise<TemplatesResult> {
  const response = await requestApi<TemplatesResponse>("/api/templates", options);
  return { templates: (response.templates ?? []) as ImportTemplate[] };
}

export async function upsertTemplate(
  template: ImportTemplateInput,
  options?: RequestOptions
): Promise<UpsertTemplateResult> {
  const response = await requestApi<UpsertTemplateResponse & { message?: string }>(
    "/api/templates/upsert",
    {
      method: "POST",
      body: JSON.stringify(template),
      ...options
    }
  );
  return {
    saved: response.saved,
    template: response.template as ImportTemplate | undefined,
    message: response.message
  };
}

export async function deleteTemplate(
  name: string,
  options?: RequestOptions
): Promise<DeleteTemplateResult> {
  return requestApi<DeleteTemplateResponse>("/api/templates/delete", {
    method: "POST",
    body: JSON.stringify({ name }),
    ...options
  });
}
