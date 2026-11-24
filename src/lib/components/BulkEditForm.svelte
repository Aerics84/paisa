<script lang="ts">
  import _ from "lodash";
  import Select from "svelte-select";

  interface Props {
    accounts: string[];
    onpreview?: (change: {
      operation: string;
      args: { oldAccountName: string; newAccountName: string };
    }) => void;
  }

  let { accounts, onpreview }: Props = $props();

  let selectItems = $derived(
    _.map(accounts, (account) => {
      return { id: account, name: account };
    })
  );

  let selectedItem = $state<{ id: string; name: string }>();

  const OPERATIONS = [{ id: "rename_account", label: "Rename Account" }];
  let selectedOperation = $state(OPERATIONS[0].id);

  let args = $state({ oldAccountName: "", newAccountName: "" });
</script>

<div class="field is-grouped">
  <div class="control">
    <div class="select">
      <select bind:value={selectedOperation}>
        {#each OPERATIONS as operation}
          <option value={operation.id}>{operation.label}</option>
        {/each}
      </select>
    </div>
  </div>
  {#if selectedOperation === "rename_account"}
    <div class="control is-expanded">
      <Select
        bind:value={selectedItem}
        showChevron={true}
        items={selectItems}
        label="name"
        itemId="id"
        placeholder="Old Account name"
        searchable={true}
        clearable={false}
        onchange={() => {
          if (!selectedItem) return;
          args.oldAccountName = selectedItem.name;
        }}
      ></Select>
    </div>
    <div class="control is-expanded">
      <input
        bind:value={args.newAccountName}
        class="input"
        type="text"
        placeholder="New Account name"
      />
    </div>
  {/if}
  <p class="control">
    <button
      class="button is-link"
      type="button"
      onclick={() => onpreview?.({ operation: selectedOperation, args: args })}
    >
      Preview
    </button>
  </p>
</div>
