<script lang="ts">
  import { ajax, configUpdated } from "$lib/utils";
  import { onMount } from "svelte";
  import type { JSONSchema7 } from "json-schema";
  import JsonSchemaForm from "$lib/components/JsonSchemaForm.svelte";
  import _ from "lodash";
  import * as toast from "bulma-toast";
  import { refresh } from "../../../../store";
  import { sync } from "$lib/sync";

  let lastConfig: UserConfig;
  let config: UserConfig;
  let schema: JSONSchema7;
  let hasChanges = true;
  let isLoading = false;
  let error: string = null;
  let accounts: string[] = [];
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
    if (
      confirm(
        "Are you sure you want to reset the config to defaults? This action is not reversible."
      )
    ) {
      save({
        journal_path: lastConfig.journal_path,
        db_path: lastConfig.db_path,
        regional_profile: lastConfig.regional_profile
      } as any);
    }
  }

  async function save(newConfig: UserConfig) {
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
        refresh();
        toast.toast({
          message: `Saved config`,
          type: "is-success"
        });

        await sync({ journal: true });
      }
    } finally {
      isLoading = false;
    }
  }

  $: hasChanges = !_.isEqual(config, lastConfig);

  $: if (config?.regional_profile && config.regional_profile !== lastAppliedRegionalProfile) {
    const defaults = regionalProfileDefaults[config.regional_profile];
    if (defaults) {
      config = { ...config, ...defaults };
    }
    lastAppliedRegionalProfile = config.regional_profile;
  }
</script>

<div class="section">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        {#if schema}
          <div class="box px-3" style="max-width: 1024px;">
            <article class="message">
              <div class="message-body">
                Prices are <b>not</b> automatically updated after config change. Use the menu at the top
                right corner to update prices. If the journal failed to sync due to any issues, fix the
                issues and use the menu to sync again.
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
                  on:click={(_e) => save(config)}
                  class="button is-success {isLoading && 'is-loading'}"
                  disabled={!hasChanges}>Save</button
                >
              </div>
              <div class="control">
                <button
                  on:click={(_e) => (config = _.cloneDeep(lastConfig))}
                  class="button is-light">Cancel</button
                >
              </div>
              <div class="control">
                <button on:click={(_e) => resetToDefault()} class="button is-danger"
                  >Reset to Defaults</button
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
