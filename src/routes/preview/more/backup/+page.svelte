<script lang="ts">
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
      const headers: Record<string, string> = {};
      const token = localStorage.getItem("token");
      if (token) {
        headers["X-Auth"] = token;
      }

      const response = await fetch(route, { headers });
      if (!response.ok) {
        throw new Error(`Failed to download backup: ${response.statusText}`);
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
      errorMessage = err instanceof Error ? err.message : "Failed to download backup";
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
            <span>Backup &amp; Export</span>
          </h1>

          <div class="content is-size-7">
            <p>
              Generate a single ZIP archive containing your ledger journals, sheets, and the
              configuration file.
            </p>
            <ul>
              <li>Includes all ledger journal files, including included files</li>
              <li>Includes <code>paisa.yaml</code></li>
              <li>Includes sheet templates and data files</li>
              <li>Excludes the SQLite cache database, which is rebuilt automatically on sync</li>
            </ul>
          </div>

          <div class="field">
            <label class="checkbox">
              <input type="checkbox" bind:checked={includeSecrets} />
              Include password hashes in <code>paisa.yaml</code>
            </label>
            <p class="help">Password values are redacted by default.</p>
          </div>

          {#if includeSecrets}
            <div class="notification is-warning is-light is-size-7 py-3">
              The exported configuration will contain the password hashes in plain text. Keep the
              archive in a safe place.
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
            <span>Download Backup (.zip)</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</section>
