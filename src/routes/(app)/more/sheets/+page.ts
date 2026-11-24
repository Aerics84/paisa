import { redirect } from "@sveltejs/kit";
import { getSheetFiles } from "$lib/api/sheets";
import type { PageLoad } from "./$types";

export const load: PageLoad = async () => {
  const { files } = await getSheetFiles();
  if (files.length > 0) {
    redirect(307, `/more/sheets/${files[0].name}`);
  }
};
