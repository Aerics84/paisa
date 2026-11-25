<script lang="ts">
  import { goto } from "$app/navigation";
  import FileModal from "$lib/components/FileModal.svelte";
  import { saveSheetFile } from "$lib/api/sheets";
  import { readonly } from "../../../../persisted_store";
  import * as toast from "bulma-toast";
  import _ from "lodash";
  import { t } from "$lib/i18n";

  let modalOpen = $state(false);
  function openCreateModal() {
    modalOpen = true;
  }

  async function createFile(destinationFile: string) {
    if ($readonly) return;
    destinationFile = destinationFile.trim() + ".paisa";
    const { saved, message } = await saveSheetFile(
      { name: destinationFile, content: "", operation: "create" },
      { background: true }
    );

    if (saved) {
      toast.toast({
        message: $t("common.createdFile", {
          url: `/more/sheets/${encodeURIComponent(destinationFile)}`,
          name: _.escape(destinationFile)
        }),
        type: "is-success",
        duration: 5000
      });

      await goto(`/more/sheets/${encodeURIComponent(destinationFile)}`);
    } else {
      toast.toast({
        message: $t("common.createFailed", { name: destinationFile, reason: message }),
        type: "is-danger",
        duration: 10000
      });
    }
  }
</script>

<FileModal
  bind:open={modalOpen}
  onsave={(detail) => createFile(detail)}
  label={$t("common.create")}
  placeholder="scratch"
  help={$t("more.sheets.filenameHelp")}
/>

<section class="section">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-6 mx-auto">
        <div class="flex items-center justify-center mt-5">
          <div class="field">
            <p class="control">
              <button
                class="button is-medium is-link"
                disabled={$readonly}
                onclick={(_e) => openCreateModal()}
              >
                <span class="icon is-small">
                  <i class="fas fa-file-circle-plus"></i>
                </span>
                <span>{$t("common.create")}</span>
              </button>
            </p>
            <p class="mt-2 has-text-grey has-text-bold">{$t("more.sheets.createFirst")}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
