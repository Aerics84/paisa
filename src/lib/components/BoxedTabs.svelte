<script lang="ts">
  import _ from "lodash";

  export let options: { label: string; value: any }[];
  export let value: any;

  $: if (value && !options.find((option) => option.value === value) && !_.isEmpty(options)) {
    value = _.last(options).value;
  }
</script>

<div class="boxed-tabs">
  {#each options as option}
    <button
      class="boxed-tab {option.value === value ? 'is-active' : ''}"
      type="button"
      on:click={() => (value = option.value)}
    >
      {option.label}
    </button>
  {/each}
</div>
