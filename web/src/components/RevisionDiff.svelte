<script lang="ts">
  import { diffWords } from "diff";

  interface Props {
    oldText: string;
    newText: string;
  }

  let { oldText, newText }: Props = $props();

  let parts = $derived(diffWords(oldText, newText));
</script>

<div class="diff">
  {#if oldText === newText}
    <p class="no-change">No text changes.</p>
  {:else}
    {#each parts as part, i (i)}
      <span class:added={part.added} class:removed={part.removed}>{part.value}</span>
    {/each}
  {/if}
</div>

<style>
  .diff {
    background: #161616;
    border: 1px solid #2a2a2a;
    border-radius: 4px;
    padding: 0.75rem;
    margin: 0.5rem 0;
    white-space: pre-wrap;
    font-size: 0.85rem;
    line-height: 1.5;
  }

  .added {
    background: #1e3a24;
    color: #8fd19e;
  }

  .removed {
    background: #3a1e1e;
    color: #d19e8f;
    text-decoration: line-through;
  }

  .no-change {
    color: #888;
    margin: 0;
  }
</style>
