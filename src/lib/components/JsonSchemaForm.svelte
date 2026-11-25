<script lang="ts">
  import type { JSONSchema7 } from "json-schema";
  import Select from "svelte-select";
  import _ from "lodash";
  import PriceCodeSearchModal from "./PriceCodeSearchModal.svelte";
  import JsonSchemaForm from "./JsonSchemaForm.svelte";
  import { iconGlyph, loadIcons } from "$lib/icon";
  import AccountSelect from "./AccountsSelect.svelte";
  import { t } from "$lib/i18n";
  import { hashPassword } from "$lib/utils";
  import { beginPasswordHash, endPasswordHash } from "$lib/password_hash";

  interface Schema extends JSONSchema7 {
    "ui:header"?: string;
    "ui:widget"?: string;
    "ui:order"?: number;
  }

  const ICON_MAX_RESULTS = 200;

  interface Props {
    key: string;
    path?: string;
    value: any;
    rawValue?: string;
    schema: Schema;
    depth?: number;
    required?: boolean;
    deletable?: (() => void) | null;
    disabled?: boolean;
    allAccounts: string[];
    modalOpen?: boolean;
  }

  let {
    key,
    path = "",
    value = $bindable(),
    rawValue = $bindable(""),
    schema,
    depth = 0,
    required = false,
    deletable = null,
    disabled = false,
    allAccounts,
    modalOpen = $bindable(false)
  }: Props = $props();

  // svelte-ignore state_referenced_locally
  let open = $state(depth < 1);
  let passwordError = $state("");
  let iconsLoading = $state(false);
  let iconsError = $state("");
  let icons: string[] | null = null;
  let iconsPromise: Promise<string[]> | null = null;
  let iconItems = $state<{ value: string; label: string }[]>([]);
  let iconQuery = "";
  let title = $derived(_.startCase(key));
  let fieldPath = $derived(path || key);
  let fieldId = $derived(`config-field-${fieldPath.replace(/[^a-zA-Z0-9_-]/g, "-")}`);

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

  function loadIconList(): Promise<string[]> {
    if (!iconsPromise) {
      iconsLoading = true;
      iconsError = "";
      iconsPromise = loadIcons()
        .then((result) => {
          icons = result;
          return result;
        })
        .catch((error) => {
          icons = null;
          iconsPromise = null;
          iconsError = $t("components.jsonSchema.iconsLoadFailed");
          console.warn("Could not load icon list", error);
          throw error;
        })
        .finally(() => {
          iconsLoading = false;
        });
    }
    return iconsPromise;
  }

  function filterIcons(all: string[], text: string): string[] {
    if (_.isEmpty(text)) {
      return _.take(all, ICON_MAX_RESULTS);
    }
    return _.take(
      all.filter((icon) => icon.includes(text)),
      ICON_MAX_RESULTS
    );
  }

  async function prepareIcons() {
    try {
      const all = await loadIconList();
      iconItems = filterIcons(all, iconQuery).map((icon) => ({ value: icon, label: icon }));
    } catch {
      // `loadIconList` records the error; keep the retry affordance visible.
    }
  }

  const ICON_OPEN_KEYS = new Set(["ArrowDown", "ArrowUp", "Enter"]);

  function openIconPicker(event?: Event) {
    if (event instanceof KeyboardEvent && !ICON_OPEN_KEYS.has(event.key)) {
      return;
    }
    if (icons || iconsPromise) {
      return;
    }
    void prepareIcons();
  }

  // Only real open interactions (click or the keys that open the dropdown)
  // may trigger the metadata download; focus/tab alone must stay cheap.
  function iconPickerTrigger(node: HTMLElement) {
    const handleClick = () => openIconPicker();
    const handleKeydown = (event: KeyboardEvent) => openIconPicker(event);
    node.addEventListener("click", handleClick);
    node.addEventListener("keydown", handleKeydown);
    return {
      destroy() {
        node.removeEventListener("click", handleClick);
        node.removeEventListener("keydown", handleKeydown);
      }
    };
  }

  async function searchIcons(text: string) {
    text = (text || "").toLowerCase();
    iconQuery = text;
    if (!icons) {
      if (_.isEmpty(text)) {
        return [];
      }
      await loadIconList();
    }
    return filterIcons(icons || [], text);
  }
</script>

{#if deletable}
  <button
    onclick={() => deletable()}
    class="config-delete"
    aria-label={$t("components.jsonSchema.deleteAria", { title })}
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
            onchange={async () => {
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
                passwordError = $t("components.jsonSchema.passwordHashFailed");
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
        <div class="control" style="max-width: 350px" use:iconPickerTrigger>
          <Select
            bind:value
            items={iconItems}
            loading={iconsLoading}
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
        {#if iconsError}
          <p class="help is-danger">
            {iconsError}
            <button type="button" class="is-link" onclick={() => openIconPicker()}
              >{$t("components.jsonSchema.retry")}</button
            >
          </p>
        {/if}
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
            {$t("components.jsonSchema.yes")}
          </label>
          <label class="radio">
            <input value="no" bind:group={value} type="radio" name="no" />
            {$t("components.jsonSchema.no")}
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
      onclick={() => (modalOpen = true)}
      class="is-link"
      aria-label={$t("components.jsonSchema.editAria", { title })}
      type="button"
    >
      <span class="icon is-small">
        <i class="fas fa-pen-to-square"></i>
      </span>
    </button>
  </div>

  <PriceCodeSearchModal
    bind:open={modalOpen}
    onselect={(selection) => {
      value["code"] = selection.code;
      value["provider"] = selection.provider;
    }}
  />

  <div class="config-body {depth % 2 == 1 ? 'odd' : 'even'}">
    {#each sortedProperties(schema) as [key, subSchema]}
      <JsonSchemaForm
        {allAccounts}
        required={_.includes(schema.required || [], key)}
        depth={depth + 1}
        {key}
        path={`${fieldPath}.${key}`}
        bind:value={value[key]}
        schema={subSchema as Schema}
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
      onclick={() => toggle()}
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
        <JsonSchemaForm
          {allAccounts}
          required={_.includes(schema.required || [], key)}
          depth={depth + 1}
          {key}
          path={`${fieldPath}.${key}`}
          bind:value={value[key]}
          schema={subSchema as Schema}
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
      onclick={() => (open = !open)}
    >
      <span>{title}</span>
      <span class="icon is-small">
        <i class="fas {open ? 'fa-angle-up' : 'fa-angle-down'}"></i>
      </span>
    </button>
    {#if open}
      <button
        onclick={() => (value = [newItem(schema), ...(value ?? [])])}
        class="config-add"
        aria-label={$t("components.jsonSchema.addAria", { title })}
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
        <JsonSchemaForm
          {allAccounts}
          deletable={() => {
            value.splice(i, 1);
            value = [...value];
          }}
          depth={depth + 1}
          key=""
          path={`${fieldPath}[${i}]`}
          bind:value={value[i]}
          schema={schema.items as Schema}
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
