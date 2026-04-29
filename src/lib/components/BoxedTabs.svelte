<script lang="ts">
  import _ from "lodash";

  export let options: { label: string; value: any }[];
  export let value: any;

  $: if (value && !options.find((option) => option.value === value) && !_.isEmpty(options)) {
    value = _.last(options).value;
  }
</script>

<div class="du-tabs du-tabs-boxed du-tabs-sm">
  {#each options as option}
    <button
      class="du-tab {option.value === value ? 'du-tab-active' : ''}"
      type="button"
      on:click={() => (value = option.value)}
    >
      {option.label}
    </button>
  {/each}
</div>
