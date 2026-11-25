<script lang="ts">
  import { t } from "$lib/i18n";
  let includeSecrets = $state(false);
  let downloading = $state(false);
  let errorMessage = $state("");

  async function downloadBackup() {
    if (typeof window === "undefined" || downloading) {
      return;
    }

    downloading = true;
    errorMessage = "";

    try {
      const route = `/api/backup/export${includeSecrets ? "?include_secrets=true" : ""}`;

      const response = await fetch(route, { credentials: "same-origin" });
      if (!response.ok) {
        throw new Error(
          $t("more.backup.downloadFailedWithStatus", { status: response.statusText })
        );
      }

      const contentDisposition = response.headers.get("Content-Disposition");
      let filename = "paisa-backup.zip";
      if (contentDisposition) {
        const match = contentDisposition.match(/filename="?([^"]+)"?/);
        if (match && match[1]) {
          filename = match[1];
        }
      }

      const blob = await response.blob();
      const blobUrl = window.URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = blobUrl;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(blobUrl);
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : $t("more.backup.downloadFailed");
    } finally {
      downloading = false;
    }
  }
</script>

<section class="section tab-backup">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-8 is-offset-2">
        <div class="box">
          <h1 class="title is-5">
            <span class="icon"><i class="fas fa-file-zipper"></i></span>
            <span>{$t("more.backup.title")}</span>
          </h1>

          <div class="content is-size-7">
            <p>{$t("more.backup.description")}</p>
            <ul>
              <li>{$t("more.backup.includesJournals")}</li>
              <li><code>paisa.yaml</code> {$t("more.backup.isIncluded")}</li>
              <li>{$t("more.backup.includesSheets")}</li>
              <li>{$t("more.backup.excludesCache")}</li>
            </ul>
          </div>

          <div class="field">
            <label class="checkbox">
              <input type="checkbox" bind:checked={includeSecrets} />
              <code>paisa.yaml</code>: {$t("more.backup.includeHashes")}
            </label>
            <p class="help">{$t("more.backup.hashesRedacted")}</p>
          </div>

          {#if includeSecrets}
            <div class="notification is-warning is-light is-size-7 py-3">
              {$t("more.backup.warningPlaintext")}
            </div>
          {/if}

          {#if errorMessage}
            <div class="notification is-danger is-light is-size-7 py-3">{errorMessage}</div>
          {/if}

          <button
            type="button"
            class="button is-primary {downloading ? 'is-loading' : ''}"
            disabled={downloading}
            onclick={downloadBackup}
          >
            <span class="icon"><i class="fas fa-download"></i></span>
            <span>{$t("more.backup.download")}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</section>
