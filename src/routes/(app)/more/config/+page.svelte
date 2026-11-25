<script lang="ts">
  import { ajax, configUpdated } from "$lib/utils";
  import { onMount } from "svelte";
  import type { JSONSchema7 } from "json-schema";
  import JsonSchemaForm from "$lib/components/JsonSchemaForm.svelte";
  import _ from "lodash";
  import * as toast from "bulma-toast";
  import { refresh } from "../../../../store";
  import { askConfirm } from "$lib/confirm";
  import { sync } from "$lib/sync";
  import { pendingPasswordHashes } from "$lib/password_hash";
  import { t } from "$lib/i18n";

  let lastConfig = $state<UserConfig>();
  let config = $state<UserConfig>();
  let schema = $state<JSONSchema7>();
  let isLoading = $state(false);
  let error = $state<string>(null);
  let accounts = $state<string[]>([]);
  let lastAppliedRegionalProfile: string = null;

  const regionalProfileDefaults: Record<string, Partial<UserConfig>> = {
    india: {
      tax_regime: "india",
      default_currency: "INR",
      display_precision: 0,
      locale: "en-IN",
      time_zone: "",
      financial_year_starting_month: 4,
      week_starting_day: 0
    },
    "germany-eu": {
      tax_regime: "germany",
      default_currency: "EUR",
      display_precision: 2,
      locale: "de-DE",
      time_zone: "Europe/Berlin",
      financial_year_starting_month: 1,
      week_starting_day: 1
    }
  };

  onMount(async () => {
    ({ config, schema, accounts } = await ajax("/api/config"));
    lastConfig = _.cloneDeep(config);
    lastAppliedRegionalProfile = config?.regional_profile || null;
  });

  async function resetToDefault() {
    const confirmed = await askConfirm({
      title: $t("settings.resetTitle"),
      description: $t("settings.resetDescription"),
      confirmLabel: $t("settings.reset"),
      cancelLabel: $t("common.cancel"),
      variant: "danger"
    });
    if (confirmed) {
      save({
        journal_path: lastConfig.journal_path,
        db_path: lastConfig.db_path,
        regional_profile: lastConfig.regional_profile
      } as any);
    }
  }

  async function save(newConfig: UserConfig) {
    if ($pendingPasswordHashes > 0) {
      return;
    }

    isLoading = true;
    try {
      let success = false;
      ({ success, error } = await ajax("/api/config", {
        method: "POST",
        body: JSON.stringify(newConfig),
        background: true
      }));

      if (success) {
        lastConfig = _.cloneDeep(newConfig);
        config = _.cloneDeep(newConfig);
        globalThis.USER_CONFIG = _.cloneDeep(newConfig);
        configUpdated();
        await refresh();
        toast.toast({
          message: $t("settings.saved"),
          type: "is-success"
        });

        await sync({ journal: true });
      }
    } finally {
      isLoading = false;
    }
  }

  let hasChanges = $derived(!_.isEqual(config, lastConfig));

  $effect(() => {
    if (config?.regional_profile && config.regional_profile !== lastAppliedRegionalProfile) {
      const defaults = regionalProfileDefaults[config.regional_profile];
      if (defaults) {
        config = { ...config, ...defaults };
      }
      lastAppliedRegionalProfile = config.regional_profile;
    }
  });
</script>

<div class="section" data-testid="config-page">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        {#if schema}
          <div class="box px-3" style="max-width: 1024px;">
            <article class="message">
              <div class="message-body">
                {$t("settings.priceNoticeBefore")} <b>{$t("settings.priceNoticeNot")}</b>
                {$t("settings.priceNoticeAfter")}
              </div>
            </article>

            {#if error}
              <article class="message is-danger">
                <div class="message-body" style="overflow: auto; white-space: pre;">
                  {error}
                </div>
              </article>
            {/if}
            <div class="field is-grouped is-grouped-right">
              <div class="control">
                <button
                  onclick={(_e) => save(config)}
                  class="button is-success {isLoading && 'is-loading'}"
                  disabled={!hasChanges || $pendingPasswordHashes > 0}>{$t("common.save")}</button
                >
                {#if $pendingPasswordHashes > 0}
                  <span class="help has-text-grey">{$t("settings.hashing")}</span>
                {/if}
              </div>
              <div class="control">
                <button onclick={(_e) => (config = _.cloneDeep(lastConfig))} class="button is-light"
                  >{$t("common.cancel")}</button
                >
              </div>
              <div class="control">
                <button onclick={(_e) => resetToDefault()} class="button is-danger"
                  >{$t("settings.resetToDefaults")}</button
                >
              </div>
            </div>
            <JsonSchemaForm
              allAccounts={accounts}
              key="configuration"
              bind:value={config}
              {schema}
            />
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>
