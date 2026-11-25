<script lang="ts">
  import { page } from "$app/state";
  import { t } from "$lib/i18n";
</script>

<section class="section" data-testid="error-page">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-4 is-offset-4 mt-5">
        {#if page.error.status === 404}
          <article class="message invertable is-danger">
            <div class="message-header">{$t("error.pageNotFound")}</div>
            <div class="message-body">
              <p>
                {$t("error.pageNotFoundText")}
                <a href="/">{$t("error.backToHomepage")}</a>.
              </p>
            </div>
          </article>
        {:else}
          <article class="message invertable is-danger">
            <div class="message-header">{$t("error.criticalTitle")}</div>
            <div class="message-body">
              <p>{$t("error.criticalText")}</p>
              <p class="mt-2">{page.error.message}</p>
              {#if page.error.stack}
                <pre class="mt-2">{page.error.stack}</pre>
              {/if}
              <p class="mt-5">
                {$t("error.reportIssue")}
                <a href="https://github.com/Aerics84/paisa/issues"
                  >https://github.com/Aerics84/paisa/issues</a
                >. {$t("error.closingMayHelp")}
              </p>
            </div>
          </article>
        {/if}
      </div>
    </div>
  </div>
</section>
