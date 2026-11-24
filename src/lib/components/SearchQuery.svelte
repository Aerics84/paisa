<script lang="ts">
  import { createEditor } from "$lib/search_query_editor";
  import type { EditorView } from "codemirror";

  interface Props {
    autocomplete: Record<string, string[]>;
  }

  let { autocomplete }: Props = $props();

  let editorDom = $state<HTMLElement>();
  let editor: EditorView;

  $effect(() => {
    if (autocomplete && editorDom) {
      if (editor) {
        editor.destroy();
      }

      editor = createEditor("", editorDom, autocomplete);
    }
  });
</script>

<div class="search-query-editor" bind:this={editorDom}></div>
