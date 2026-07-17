<script lang="ts">
  import { tick } from "svelte";
  import { slide } from "svelte/transition";
  import * as api from "./lib/api";
  import type { PageSummary, Page, Revision, SearchResult } from "./types/Page";
  import PageTree from "./components/PageTree.svelte";
  import Editor from "./components/Editor.svelte";
  import RevisionDiff from "./components/RevisionDiff.svelte";

  let pages = $state<PageSummary[]>([]);
  let selectedId = $state<string | null>(null);
  let current = $state<Page | null>(null);

  let draggedId = $state<string | null>(null);
  let topLevelDropActive = $state(false);

  // Off-canvas drawer state. Ignored above the mobile breakpoint (sidebar is always visible there).
  let sidebarOpen = $state(false);

  let titleInputEl = $state<HTMLInputElement | null>(null);
  let editorComponent = $state<ReturnType<typeof Editor> | null>(null);

  // Mobile-only: secondary toolbar actions (move, autosave, save, history, delete)
  // collapse behind this toggle so the persistent chrome stays to one slim row.
  let moreOpen = $state(false);

  let editTitle = $state("");
  let editParentId = $state<string | null>(null);
  let editContentJson = $state("");
  let editContentText = $state("");
  let saving = $state(false);
  let error = $state<string | null>(null);

  // Derived from actual content, not an edit flag — so a no-op edit (type a space, delete it) isn't "dirty".
  let dirty = $derived(
    current !== null &&
      (editTitle !== current.title ||
        editParentId !== current.parentId ||
        editContentJson !== current.contentJson),
  );

  let searchQuery = $state("");
  let searchResults = $state<SearchResult[]>([]);

  let revisions = $state<Revision[]>([]);
  let showRevisions = $state(false);

  let autosaveEnabled = $state(localStorage.getItem("palimpsest-autosave") === "true");
  let autosaveIntervalMs = $state(10_000); // default until /api/config loads

  async function loadConfig() {
    try {
      const cfg = await api.getConfig();
      if (cfg.autosaveIntervalSeconds > 0) {
        autosaveIntervalMs = cfg.autosaveIntervalSeconds * 1000;
      }
    } catch {}
  }

  $effect(() => {
    localStorage.setItem("palimpsest-autosave", String(autosaveEnabled));
    if (!autosaveEnabled) return;
    const ms = autosaveIntervalMs;
    const interval = setInterval(() => {
      if (dirty && !saving) save();
    }, ms);
    return () => clearInterval(interval);
  });

  async function loadPages() {
    pages = await api.listPages();
  }

  // True if safe to navigate away: nothing unsaved, autosave saved it, or the user chose to discard.
  async function resolveUnsavedChanges(): Promise<boolean> {
    if (!dirty) return true;
    if (autosaveEnabled) {
      // Autosave implies "always keep my work" — save rather than prompt.
      return await save();
    }
    return confirm("Discard unsaved changes?");
  }

  async function loadPageIntoEditor(id: string) {
    showRevisions = false;
    error = null;
    current = await api.getPage(id);
    selectedId = id;
    editTitle = current.title;
    editParentId = current.parentId;
    editContentJson = current.contentJson;
    editContentText = current.contentText;
    sidebarOpen = false; // no-op on desktop; on mobile, picking a page should hand focus to the editor
    moreOpen = false;
  }

  async function selectPage(id: string) {
    if (!(await resolveUnsavedChanges())) return;
    try {
      await loadPageIntoEditor(id);
      await tick();
      editorComponent?.focus();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  function onEditorChange(contentJson: string, contentText: string) {
    editContentJson = contentJson;
    editContentText = contentText;
  }

  // Plain string DOM values + onchange, not bind:value — avoids `null` vs option-value matching issues.
  function onParentSelectChange(e: Event) {
    const value = (e.currentTarget as HTMLSelectElement).value;
    editParentId = value === "" ? null : value;
  }

  // Excludes this page and its own descendants (backend also enforces this).
  interface MoveTarget {
    page: PageSummary;
    depth: number;
  }

  // Tree order + depth (backend returns a flat alphabetical list).
  function moveTargets(pageId: string): MoveTarget[] {
    const excluded = new Set<string>([pageId]);
    let grew = true;
    while (grew) {
      grew = false;
      for (const p of pages) {
        if (p.parentId !== null && excluded.has(p.parentId) && !excluded.has(p.id)) {
          excluded.add(p.id);
          grew = true;
        }
      }
    }
    const eligible = pages.filter((p) => !excluded.has(p.id));

    const result: MoveTarget[] = [];
    function walk(parentId: string | null, depth: number) {
      for (const p of eligible.filter((p) => p.parentId === parentId)) {
        result.push({ page: p, depth });
        walk(p.id, depth + 1);
      }
    }
    walk(null, 0);
    return result;
  }

  function moveTargetLabel(target: MoveTarget): string {
    const indent = "  ".repeat(target.depth);
    const marker = target.depth > 0 ? "↳ " : "";
    return `${indent}${marker}${target.page.title}`;
  }

  async function save(): Promise<boolean> {
    if (!current) return true;
    saving = true;
    error = null;
    try {
      current = await api.updatePage(
        current.id,
        editTitle,
        editParentId,
        editContentJson,
        editContentText,
      );
      await loadPages();
      return true;
    } catch (e) {
      error = (e as Error).message;
      return false;
    } finally {
      saving = false;
    }
  }

  function startDrag(id: string) {
    draggedId = id;
  }

  function endDrag() {
    draggedId = null;
    topLevelDropActive = false;
  }

  async function movePage(id: string, newParentId: string | null) {
    try {
      const moved = await api.movePage(id, newParentId);
      if (current?.id === id) {
        current = moved;
        editParentId = moved.parentId;
      }
      await loadPages();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  async function createPage() {
    if (!(await resolveUnsavedChanges())) return;
    const parentId = current?.id ?? null;
    try {
      const page = await api.createPage(parentId, "Untitled");
      await loadPages();
      await loadPageIntoEditor(page.id);
      await tick();
      titleInputEl?.focus();
      titleInputEl?.select();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  async function removePage() {
    if (!current) return;
    if (!confirm(`Delete "${current.title}"? This cannot be undone.`)) return;
    try {
      await api.deletePage(current.id);
      current = null;
      selectedId = null;
      await loadPages();
    } catch (e) {
      error = (e as Error).message;
    }
  }

  async function toggleRevisions() {
    if (!current) return;
    if (!showRevisions) {
      revisions = await api.listRevisions(current.id);
      expandedDiffId = null;
    }
    showRevisions = !showRevisions;
  }

  let expandedDiffId = $state<string | null>(null);

  function toggleDiff(revisionId: string) {
    expandedDiffId = expandedDiffId === revisionId ? null : revisionId;
  }

  // revisions is newest-first pre-save snapshots; successor is the next-newer one, or live content for the newest.
  function successorTextFor(index: number): string {
    if (index === 0) return current?.contentText ?? "";
    return revisions[index - 1].contentText;
  }

  async function revertTo(revisionId: string) {
    if (!current) return;
    if (!confirm("Revert to this revision? Current content will be saved as a new revision.")) return;
    current = await api.revertToRevision(current.id, revisionId);
    editTitle = current.title;
    editParentId = current.parentId;
    editContentJson = current.contentJson;
    editContentText = current.contentText;
    showRevisions = false;
    await loadPages();
  }

  let searchTimer: ReturnType<typeof setTimeout>;
  function onSearchInput() {
    clearTimeout(searchTimer);
    if (!searchQuery.trim()) {
      searchResults = [];
      return;
    }
    searchTimer = setTimeout(async () => {
      searchResults = await api.search(searchQuery);
    }, 250);
  }

  function selectFromSearch(id: string) {
    searchQuery = "";
    searchResults = [];
    selectPage(id);
  }

  loadPages();
  loadConfig();
</script>

<div class="layout">
  <div class="mobile-topbar">
    <button class="menu-toggle" onclick={() => (sidebarOpen = true)} aria-label="Show notes list">
      <span></span><span></span><span></span>
    </button>
    {#if !current}
      <span class="mobile-title">Palimpsest</span>
    {/if}
  </div>

  {#if sidebarOpen}
    <div
      class="backdrop"
      role="button"
      tabindex="-1"
      aria-label="Close notes list"
      onclick={() => (sidebarOpen = false)}
      onkeydown={(e) => e.key === "Escape" && (sidebarOpen = false)}
    ></div>
  {/if}

  <aside class="sidebar" class:open={sidebarOpen}>
    <div class="search">
      <input
        type="text"
        placeholder="Search..."
        bind:value={searchQuery}
        oninput={onSearchInput}
      />
      {#if searchResults.length > 0}
        <ul class="search-results">
          {#each searchResults as r (r.id)}
            <li>
              <button onclick={() => selectFromSearch(r.id)}>
                <strong>{r.title}</strong>
                <span class="snippet">{@html r.snippet}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <button class="new-page" onclick={createPage}>+ New page</button>

    <nav
      ondragover={(e) => {
        if (draggedId === null) return;
        e.preventDefault();
      }}
      ondrop={(e) => {
        e.preventDefault();
        if (draggedId !== null) movePage(draggedId, null);
      }}
    >
      <PageTree
        {pages}
        parentId={null}
        {selectedId}
        {draggedId}
        onSelect={selectPage}
        onDragStart={startDrag}
        onDragEnd={endDrag}
        onDrop={movePage}
      />
    </nav>

    {#if draggedId !== null}
      <div
        class="drop-top-level"
        role="region"
        aria-label="Drop zone to move page to top level"
        class:active={topLevelDropActive}
        ondragover={(e) => {
          e.preventDefault();
          e.stopPropagation();
          topLevelDropActive = true;
        }}
        ondragleave={() => (topLevelDropActive = false)}
        ondrop={(e) => {
          e.preventDefault();
          e.stopPropagation();
          topLevelDropActive = false;
          if (draggedId !== null) movePage(draggedId, null);
        }}
        transition:slide={{ duration: 150 }}
      >
        Move to top level
      </div>
    {/if}
  </aside>

  <main>
    {#if current}
      <div class="toolbar">
        <input
          class="title-input"
          type="text"
          bind:this={titleInputEl}
          bind:value={editTitle}
          placeholder="Page title"
        />
        <button
          class="overflow-toggle"
          onclick={() => (moreOpen = !moreOpen)}
          aria-expanded={moreOpen}
          aria-label={moreOpen ? "Hide page actions" : "Show page actions"}
        >
          <span></span><span></span><span></span>
        </button>
        <div class="toolbar-actions" class:open={moreOpen}>
          <label class="move-control">
            <span>Move to</span>
            <select
              class="parent-select"
              value={editParentId ?? ""}
              onchange={onParentSelectChange}
            >
              <option value="">— Top level —</option>
              {#each moveTargets(current.id) as target (target.page.id)}
                <option value={target.page.id}>{moveTargetLabel(target)}</option>
              {/each}
            </select>
          </label>
          <label class="autosave-toggle">
            <input type="checkbox" bind:checked={autosaveEnabled} />
            Autosave
          </label>
          <button class="btn-primary" onclick={save} disabled={!dirty || saving}>
            {saving ? "Saving..." : "Save"}
          </button>
          <button onclick={toggleRevisions}>History</button>
          <button class="danger" onclick={removePage}>Delete</button>
        </div>
      </div>

      {#if error}
        <p class="error">{error}</p>
      {/if}

      {#if showRevisions}
        <div class="revisions">
          <h3>Revision history</h3>
          {#if revisions.length === 0}
            <p>No previous revisions.</p>
          {:else}
            <ul>
              {#each revisions as rev, i (rev.id)}
                <li class="revision-row">
                  <div class="revision-header">
                    <span><span class="revision-time">{new Date(rev.createdAt).toLocaleString()}</span> — {rev.title}</span>
                    <span class="revision-actions">
                      <button onclick={() => toggleDiff(rev.id)}>
                        {expandedDiffId === rev.id ? "Hide diff" : "Diff"}
                      </button>
                      <button onclick={() => revertTo(rev.id)}>Revert</button>
                    </span>
                  </div>
                  {#if expandedDiffId === rev.id}
                    <RevisionDiff oldText={rev.contentText} newText={successorTextFor(i)} />
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {:else}
        <Editor
          bind:this={editorComponent}
          contentJson={editContentJson}
          {pages}
          onChange={onEditorChange}
          onNavigateToPage={selectPage}
        />
      {/if}
    {:else}
      <div class="empty-state">
        <p>Select a page, or create a new one to get started.</p>
      </div>
    {/if}
  </main>
</div>

<style>
  .layout {
    display: flex;
    height: 100vh;
  }

  .sidebar {
    width: 280px;
    flex-shrink: 0;
    border-right: 1px solid var(--border);
    padding: 1rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .mobile-topbar {
    display: none;
  }

  .menu-toggle {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    gap: 3px;
    width: 2.25rem;
    height: 2.25rem;
    padding: 0;
  }

  .menu-toggle span {
    display: block;
    width: 16px;
    height: 1.5px;
    background: currentColor;
  }

  .backdrop {
    display: none;
  }

  .search {
    position: relative;
  }

  .search input {
    width: 100%;
    padding: 0.4rem 0.6rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: inherit;
    font-family: inherit;
  }

  .search-results {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    z-index: 10;
    background: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: 4px;
    margin-top: 0.25rem;
    max-height: 300px;
    overflow-y: auto;
    list-style: none;
    padding: 0.25rem;
  }

  .search-results button {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: inherit;
    padding: 0.4rem;
    cursor: pointer;
    border-radius: 3px;
  }

  .search-results button:hover {
    background: var(--surface);
  }

  .search-results .snippet {
    display: block;
    font-size: 0.8rem;
    color: var(--muted);
  }

  .search-results :global(mark) {
    background: var(--accent-tint);
    color: var(--accent);
  }

  .new-page {
    background: transparent;
    border: 1px solid var(--accent);
    color: var(--accent);
    padding: 0.5rem;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 500;
    transition: background-color 120ms ease;
  }

  .new-page:hover {
    background: var(--accent-tint);
  }

  .drop-top-level {
    flex-shrink: 0;
    text-align: center;
    padding: 0.5rem 0.5rem 0.4rem;
    border-top: 1px solid var(--border);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.7rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    transition:
      background-color 120ms ease,
      border-color 120ms ease,
      color 120ms ease;
  }

  .drop-top-level.active {
    background: var(--accent-tint);
    border-color: var(--accent);
    color: var(--accent);
  }

  main {
    flex: 1;
    padding: 1.5rem 2rem;
    overflow-y: auto;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 1rem;
    position: sticky;
    top: 0;
    background: var(--bg);
    padding-bottom: 0.75rem;
    z-index: 1;
  }

  .title-input {
    flex: 1;
    min-width: 0;
    font-size: 1.4rem;
    font-family: inherit;
    background: none;
    border: none;
    border-bottom: 1px solid transparent;
    color: inherit;
    padding: 0.25rem 0;
  }

  .title-input:focus {
    outline: none;
    border-bottom-color: var(--accent);
  }

  .overflow-toggle {
    display: none;
    flex-shrink: 0;
    flex-direction: row;
    justify-content: center;
    align-items: center;
    gap: 3px;
    width: 2.25rem;
    height: 2.25rem;
    padding: 0;
  }

  .overflow-toggle span {
    display: block;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
  }

  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .move-control {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    color: var(--muted);
    white-space: nowrap;
  }

  .parent-select {
    background: var(--surface);
    border: 1px solid var(--border);
    color: inherit;
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    max-width: 180px;
    font-family: inherit;
  }

  .autosave-toggle {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
    color: var(--muted);
    white-space: nowrap;
    cursor: pointer;
  }

  button {
    background: var(--surface);
    border: 1px solid var(--border);
    color: inherit;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
    font-family: inherit;
    transition:
      background-color 120ms ease,
      border-color 120ms ease;
  }

  button:hover:not(:disabled) {
    border-color: var(--accent);
  }

  button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  button.btn-primary:not(:disabled) {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--bg);
    font-weight: 500;
  }

  button.btn-primary:hover:not(:disabled) {
    filter: brightness(1.08);
  }

  button.danger {
    background: var(--danger-tint);
    border-color: transparent;
    color: var(--danger);
  }

  button.danger:hover:not(:disabled) {
    border-color: var(--danger);
  }

  .empty-state {
    color: var(--muted);
    margin-top: 3rem;
    text-align: center;
  }

  .error {
    color: var(--danger);
  }

  .revisions ul {
    list-style: none;
    padding: 0;
  }

  .revision-row {
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--border);
  }

  .revision-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .revision-time {
    font-family: var(--font-mono);
    font-size: 0.85rem;
    color: var(--muted);
  }

  .revision-actions {
    display: flex;
    gap: 0.4rem;
    flex-shrink: 0;
  }

  @media (max-width: 720px) {
    .layout {
      position: relative;
      flex-direction: column;
    }

    .mobile-topbar {
      display: flex;
      align-items: center;
      gap: 0.6rem;
      flex-shrink: 0;
      padding: 0.4rem 0.6rem;
      border-bottom: 1px solid var(--border);
      background: var(--bg);
      min-height: 0;
    }

    .mobile-topbar:has(.mobile-title) {
      padding: 0.5rem 0.75rem;
    }

    .mobile-title {
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .backdrop {
      display: block;
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      z-index: 25;
      border: none;
      padding: 0;
    }

    .sidebar {
      position: fixed;
      top: 0;
      bottom: 0;
      left: 0;
      z-index: 30;
      width: 85%;
      max-width: 320px;
      background: var(--bg);
      transform: translateX(-100%);
      transition: transform 200ms ease;
      box-shadow: 4px 0 24px rgba(0, 0, 0, 0.4);
    }

    .sidebar.open {
      transform: translateX(0);
    }

    main {
      padding: 0.75rem;
    }

    .toolbar {
      flex-wrap: wrap;
      gap: 0.4rem;
      margin-bottom: 0.5rem;
      padding-bottom: 0.4rem;
    }

    .title-input {
      font-size: 1.15rem;
      padding: 0.15rem 0;
    }

    .overflow-toggle {
      display: flex;
    }

    /* Collapsed by default — moving/saving/history/delete are secondary to reading
       and writing, so they live behind the toggle instead of costing a permanent
       chunk of the viewport. */
    .toolbar-actions {
      display: none;
      flex-basis: 100%;
      flex-wrap: wrap;
      gap: 0.5rem;
      padding-top: 0.5rem;
    }

    .toolbar-actions.open {
      display: flex;
    }

    .move-control {
      flex-basis: 100%;
    }

    .parent-select {
      max-width: none;
      flex: 1;
    }

    /* Touch targets grow to a comfortable minimum; drag-and-drop isn't available on
       touch, so moving a page happens through this select instead — it needs to be
       easy to hit. */
    .parent-select,
    button {
      min-height: 44px;
    }
  }
</style>
