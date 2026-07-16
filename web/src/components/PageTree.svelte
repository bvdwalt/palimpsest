<script lang="ts">
  import type { PageSummary } from "../types/Page";
  import PageTree from "./PageTree.svelte";

  interface Props {
    pages: PageSummary[];
    parentId: string | null;
    selectedId: string | null;
    onSelect: (id: string) => void;
  }

  let { pages, parentId, selectedId, onSelect }: Props = $props();

  let children = $derived(pages.filter((p) => p.parentId === parentId));
</script>

<ul class="tree">
  {#each children as page (page.id)}
    <li>
      <button
        class="node"
        class:selected={page.id === selectedId}
        onclick={() => onSelect(page.id)}
      >
        {page.title}
      </button>
      {#if pages.some((p) => p.parentId === page.id)}
        <PageTree {pages} parentId={page.id} {selectedId} {onSelect} />
      {/if}
    </li>
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding-left: 0.9rem;
  }

  .tree:first-child {
    padding-left: 0;
  }

  .node {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: #ccc;
    padding: 0.3rem 0.5rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  .node:hover {
    background: #2a2a2a;
  }

  .node.selected {
    background: #2d4a63;
    color: #fff;
  }
</style>
