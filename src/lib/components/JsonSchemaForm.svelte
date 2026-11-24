<script lang="ts">
  import type { JSONSchema7 } from "json-schema";
  import Select from "svelte-select";
  import _ from "lodash";
  import PriceCodeSearchModal from "./PriceCodeSearchModal.svelte";
  import { iconGlyph, iconsList } from "$lib/icon";
  import AccountSelect from "./AccountsSelect.svelte";
  import { hashPassword } from "$lib/utils";
  import { beginPasswordHash, endPasswordHash } from "$lib/password_hash";

  interface Schema extends JSONSchema7 {
    "ui:header"?: string;
    "ui:widget"?: string;
    "ui:order"?: number;
  }

  const ICON_MAX_RESULTS = 200;

  export let key: string;
  export let path: string = "";
  export let value: any;
  export let rawValue: string = "";
  export let schema: Schema;
  export let depth: number = 0;
  export let required = false;
  export let deletable: () => void = null;
  export let disabled: boolean = false;
  export let allAccounts: string[];

  export let modalOpen = false;

  let open = depth < 1;
  let passwordError = "";
  $: title = _.startCase(key);
  $: fieldPath = path || key;
  $: fieldId = `config-field-${fieldPath.replace(/[^a-zA-Z0-9_-]/g, "-")}`;

  function newItem(schema: any) {
    if (Array.isArray(schema.default) && schema.default.length > 0) {
      return _.cloneDeep(schema.default[0]);
    }

    switch (schema.items?.type) {
      case "object":
        return {};
      case "string":
        return "";
      case "integer":
      case "number":
        return 0;
      case "boolean":
        return false;
      default:
        return {};
    }
  }

  function sortedProperties(schema: Schema) {
    return _.sortBy(Object.entries(schema.properties), ([key, subSchema]: [string, Schema]) => {
      return [
        subSchema["ui:order"] || 999,
        _.includes(schema.required || [], key) ? 0 : 1,
        subSchema.type == "object" ? 2 : subSchema.type == "array" ? 3 : 1,
        key
      ];
    });
  }

  function toggle() {
    open = !open;
    if (!open) return;
    if (value == null || typeof value !== "object") {
      value = {};
    }
    for (const [childKey, childSchema] of sortedProperties(schema) as [string, Schema][]) {
      if (
        childSchema.type === "object" &&
        (value[childKey] == null || typeof value[childKey] !== "object")
      ) {
        value[childKey] = {};
      } else if (
        childSchema.type === "array" &&
        Array.isArray(childSchema.default) &&
        !Array.isArray(value[childKey])
      ) {
        value[childKey] = _.cloneDeep(childSchema.default);
      }
    }
  }

  function documentation(schema: Schema) {
    if (schema.description) {
      return `<p style="max-width: 300px">${schema.description}</p>`;
    }
    return null;
  }

  async function searchIcons(text: string) {
    text = text.toLowerCase();
    if (_.isEmpty(text)) {
      return _.take(iconsList, ICON_MAX_RESULTS);
    }
    return _.take(
      iconsList.filter((icon) => icon.includes(text)),
      ICON_MAX_RESULTS
    );
  }
</script>

{#if deletable}
  <button
    on:click={(_e) => deletable()}
    class="config-delete"
    aria-label={`Delete ${title}`}
    type="button"
  >
    <span class="icon is-small">
      <i class="fas fa-circle-minus"></i>
    </span>
  </button>
{/if}

{#if schema["ui:widget"] == "hidden"}
  <div></div>
{:else if schema["ui:widget"] == "password"}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <label for={fieldId} data-tippy-content={documentation(schema)} class="label">{title}</label>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control">
          <input
            {disabled}
            {required}
            class="input is-small"
            style="max-width: 350px;"
            type="password"
            id={fieldId}
            aria-label={title}
            bind:value={rawValue}
            on:change={async () => {
              passwordError = "";

              if (_.isEmpty(rawValue)) {
                return;
              }

              beginPasswordHash();
              try {
                value = await hashPassword(rawValue);
              } catch {
                // Keep the previously stored hash when hashing fails; the user
                // can retry without saving a plaintext or undefined value.
                passwordError = "Could not hash password; previous value kept";
              } finally {
                endPasswordHash();
              }
            }}
          />
        </div>
        {#if passwordError}
          <p class="help is-danger">{passwordError}</p>
        {/if}
      </div>
    </div>
  </div>
{:else if schema["ui:widget"] == "icon"}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <span data-tippy-content={documentation(schema)} class="label">{title}</span>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control" style="max-width: 350px">
          <Select
            bind:value
            class="icon-select is-small"
            showChevron={true}
            loadOptions={searchIcons}
            searchable={true}
            clearable={!required}
            inputAttributes={{ "aria-label": title }}
          >
            {#snippet selection({ selection })}
              <div class="custom-icon">
                <span>{iconGlyph(selection.value as string)} {selection.value}</span>
              </div>
            {/snippet}
            {#snippet item({ item })}
              <div class="custom-icon">
                <span class="name">{iconGlyph(item.value as string)} {item.value}</span>
              </div>
            {/snippet}
          </Select>
        </div>
      </div>
    </div>
  </div>
{:else if schema["ui:widget"] == "boolean"}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <span data-tippy-content={documentation(schema)} class="label">{title}</span>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control">
          <label class="radio">
            <input value="yes" bind:group={value} type="radio" name="yes" />
            Yes
          </label>
          <label class="radio">
            <input value="no" bind:group={value} type="radio" name="no" />
            No
          </label>
        </div>
      </div>
    </div>
  </div>
{:else if schema.type === "string" || _.isEqual(schema.type, ["string", "integer"])}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <label for={fieldId} data-tippy-content={documentation(schema)} class="label">{title}</label>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control">
          {#if schema.enum}
            <div class="select is-small">
              <select {disabled} bind:value {required} id={fieldId} aria-label={title}>
                {#each schema.enum as option}
                  <option value={option}>{option}</option>
                {/each}
              </select>
            </div>
          {:else if schema["ui:widget"] == "textarea"}
            <textarea
              {disabled}
              {required}
              class="textarea is-small"
              style="min-width: 350px;max-width: 350px; width: 350px;"
              rows="5"
              id={fieldId}
              aria-label={title}
              bind:value
              spellcheck="false"
              data-enable-grammarly="false"></textarea>
          {:else}
            <input
              {disabled}
              {required}
              pattern={schema.pattern}
              class="input is-small"
              style="max-width: 350px;"
              type="text"
              id={fieldId}
              aria-label={title}
              bind:value
            />
          {/if}
        </div>
      </div>
    </div>
  </div>
{:else if schema.type === "integer" || schema.type === "number"}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <label for={fieldId} data-tippy-content={documentation(schema)} class="label">{title}</label>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control">
          <input
            {required}
            class="input is-small"
            style="max-width: 350px;"
            type="number"
            min={schema.minimum}
            max={schema.maximum}
            step={schema.type == "integer" ? 1 : 0.01}
            id={fieldId}
            aria-label={title}
            bind:value
          />
        </div>
      </div>
    </div>
  </div>
{:else if schema["ui:widget"] == "accounts"}
  <div class="field is-horizontal">
    <div class="field-label is-small">
      <span data-tippy-content={documentation(schema)} class="label">{title}</span>
    </div>
    <div class="field-body">
      <div class="field">
        <div class="control pr-5">
          <AccountSelect {allAccounts} bind:accounts={value} />
        </div>
      </div>
    </div>
  </div>
{:else if schema["ui:widget"] == "price"}
  <div class="config-header">
    <span class="is-link" data-tippy-content={documentation(schema)}>
      <span>{title}</span>
    </span>

    <button
      on:click={(_e) => (modalOpen = true)}
      class="is-link"
      aria-label={`Edit ${title}`}
      type="button"
    >
      <span class="icon is-small">
        <i class="fas fa-pen-to-square"></i>
      </span>
    </button>
  </div>

  <PriceCodeSearchModal
    bind:open={modalOpen}
    on:select={(e) => {
      value["code"] = e.detail.code;
      value["provider"] = e.detail.provider;
    }}
  />

  <div class="config-body {depth % 2 == 1 ? 'odd' : 'even'}">
    {#each sortedProperties(schema) as [key, subSchema]}
      <svelte:self
        {allAccounts}
        required={_.includes(schema.required || [], key)}
        depth={depth + 1}
        {key}
        path={`${fieldPath}.${key}`}
        bind:value={value[key]}
        schema={subSchema}
        disabled={true}
      />
    {/each}
  </div>
{:else if schema.type == "object"}
  <div class="config-header">
    <button
      class="is-link is-light invertable"
      data-tippy-content={documentation(schema)}
      type="button"
      on:click={(_e) => toggle()}
    >
      <span>{schema["ui:header"] ? value[schema["ui:header"]] || title : title}</span>
      <span class="icon is-small">
        <i class="fas {open ? 'fa-angle-up' : 'fa-angle-down'}"></i>
      </span>
    </button>
  </div>

  {#if open}
    <div class="config-body {depth % 2 == 1 ? 'odd' : 'even'}">
      {#each sortedProperties(schema) as [key, subSchema]}
        <svelte:self
          {allAccounts}
          required={_.includes(schema.required || [], key)}
          depth={depth + 1}
          {key}
          path={`${fieldPath}.${key}`}
          bind:value={value[key]}
          schema={subSchema}
        />
      {/each}
    </div>
  {/if}
{:else if schema.type == "array"}
  <div class="config-header">
    <button
      class="is-link is-light invertable"
      data-tippy-content={documentation(schema)}
      type="button"
      on:click={(_e) => (open = !open)}
    >
      <span>{title}</span>
      <span class="icon is-small">
        <i class="fas {open ? 'fa-angle-up' : 'fa-angle-down'}"></i>
      </span>
    </button>
    {#if open}
      <button
        on:click={(_e) => (value = [newItem(schema), ...(value ?? [])])}
        class="config-add"
        aria-label={`Add ${title}`}
        type="button"
      >
        <span class="icon is-small">
          <i class="fas fa-circle-plus"></i>
        </span>
      </button>
    {/if}
  </div>

  {#if open}
    <div class="config-body {depth % 2 == 1 ? 'odd' : 'even'}">
      {#each value as _item, i}
        <svelte:self
          {allAccounts}
          deletable={() => {
            value.splice(i, 1);
            value = [...value];
          }}
          depth={depth + 1}
          key=""
          path={`${fieldPath}[${i}]`}
          bind:value={value[i]}
          schema={schema.items}
        />
      {/each}
    </div>
  {/if}
{:else}
  <div>{JSON.stringify(schema)}</div>
{/if}

<style lang="scss">
  button.is-link,
  .config-add,
  .config-delete {
    background: none;
    border: none;
    padding: 0;
  }
</style>
