import { redirect } from "@sveltejs/kit";
import { getEditorFiles } from "$lib/api/editor";
import type { PageLoad } from "./$types";

export const load: PageLoad = async () => {
  const { files } = await getEditorFiles();
  redirect(307, `/ledger/editor/${encodeURIComponent(files[0].name)}`);
};
