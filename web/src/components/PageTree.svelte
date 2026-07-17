<script lang="ts">
  import type { PageSummary } from "../types/Page";
  import PageTree from "./PageTree.svelte";

  interface Props {
    pages: PageSummary[];
    parentId: string | null;
    selectedId: string | null;
    draggedId: string | null;
    onSelect: (id: string) => void;
    onDragStart: (id: string) => void;
    onDragEnd: () => void;
    onDrop: (id: string, newParentId: string) => void;
  }

  let { pages, parentId, selectedId, draggedId, onSelect, onDragStart, onDragEnd, onDrop }: Props =
    $props();

  let children = $derived(pages.filter((p) => p.parentId === parentId));
  let isNested = $derived(parentId !== null);

  let dragOverId = $state<string | null>(null);
  let invalidHoverId = $state<string | null>(null);

  // Local hover state can outlive the drag if dragleave doesn't fire (e.g. drag cancelled via Escape).
  $effect(() => {
    if (draggedId === null) {
      dragOverId = null;
      invalidHoverId = null;
    }
  });

  // A page can't become its own parent, or the parent of one of its own descendants.
  function isInvalidTarget(targetId: string): boolean {
    if (draggedId === null) return false;
    if (targetId === draggedId) return true;
    let current: PageSummary | undefined = pages.find((p) => p.id === targetId);
    while (current) {
      if (current.id === draggedId) return true;
      const parentId: string | null = current.parentId;
      current = parentId ? pages.find((p) => p.id === parentId) : undefined;
    }
    return false;
  }

  function handleDragStart(e: DragEvent, id: string) {
    e.dataTransfer?.setData("text/plain", id);
    if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
    onDragStart(id);
  }

  function handleDragOver(e: DragEvent, targetId: string) {
    if (isInvalidTarget(targetId)) {
      invalidHoverId = targetId;
      return;
    }
    invalidHoverId = null;
    e.preventDefault();
    e.stopPropagation();
    dragOverId = targetId;
  }

  function handleDragLeave(targetId: string) {
    if (dragOverId === targetId) dragOverId = null;
    if (invalidHoverId === targetId) invalidHoverId = null;
  }

  function handleDrop(e: DragEvent, targetId: string) {
    e.preventDefault();
    e.stopPropagation();
    dragOverId = null;
    if (draggedId === null || isInvalidTarget(targetId)) return;
    onDrop(draggedId, targetId);
  }
</script>

<ul class="tree" class:nested={isNested}>
  {#each children as page (page.id)}
    <li>
      <button
        class="node"
        class:selected={page.id === selectedId}
        class:dragging={page.id === draggedId}
        class:drop-target={dragOverId === page.id}
        class:invalid-target={invalidHoverId === page.id}
        draggable="true"
        onclick={() => onSelect(page.id)}
        ondragstart={(e) => handleDragStart(e, page.id)}
        ondragend={onDragEnd}
        ondragover={(e) => handleDragOver(e, page.id)}
        ondragleave={() => handleDragLeave(page.id)}
        ondrop={(e) => handleDrop(e, page.id)}
      >
        <span class="grip" aria-hidden="true"></span>
        <span class="label">{page.title}</span>
      </button>
      {#if pages.some((p) => p.parentId === page.id)}
        <PageTree
          {pages}
          parentId={page.id}
          {selectedId}
          {draggedId}
          {onSelect}
          {onDragStart}
          {onDragEnd}
          {onDrop}
        />
      {/if}
    </li>
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding-left: 0;
  }

  .tree.nested {
    padding-left: 0.9rem;
    border-left: 1px solid var(--border);
    margin-left: 0.6rem;
  }

  .node {
    display: flex;
    align-items: center;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    border-left: 2px solid transparent;
    color: var(--muted);
    padding: 0.3rem 0.5rem;
    border-radius: 0 4px 4px 0;
    cursor: grab;
    font-size: 0.9rem;
    transition:
      background-color 120ms ease,
      border-color 120ms ease,
      box-shadow 120ms ease,
      transform 120ms ease,
      opacity 120ms ease;
  }

  .node .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* A minimal grip glyph: a 2x3 dot grid drawn from a tiling radial-gradient, no icon font needed. */
  .node .grip {
    flex-shrink: 0;
    width: 8px;
    height: 12px;
    margin-right: 0.45rem;
    background-image: radial-gradient(circle, currentColor 1px, transparent 1.4px);
    background-size: 4px 4px;
    opacity: 0;
    transition: opacity 120ms ease;
  }

  .node:hover .grip {
    opacity: 0.4;
  }

  .node:hover {
    background: var(--surface);
    color: var(--text);
  }

  .node.selected {
    background: var(--accent-tint);
    border-left-color: var(--accent);
    color: var(--text);
  }

  .node.dragging {
    opacity: 0.35;
    cursor: grabbing;
  }

  .node.dragging .grip {
    opacity: 0.6;
  }

  /* Deliberately not a filled/bordered treatment like .selected: a ring reads as "proposed",
     a fill reads as "current" — the two states need to stay visually distinct. */
  .node.drop-target {
    box-shadow: inset 0 0 0 1.5px var(--accent);
    color: var(--text);
    transform: translateX(3px);
  }

  .node.invalid-target {
    opacity: 0.5;
  }

  @media (pointer: coarse) {
    /* No touch equivalent for HTML5 drag-and-drop, so the grab cursor and grip
       glyph would advertise a gesture that doesn't work — moving a page on
       these devices happens through the "Move to" select in the toolbar instead. */
    .node {
      cursor: default;
    }

    .node .grip {
      display: none;
    }
  }
</style>
