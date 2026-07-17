<script lang="ts">
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

  let autosaveEnabled = $state(localStorage.getItem("inkbase-autosave") === "true");
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
    localStorage.setItem("inkbase-autosave", String(autosaveEnabled));
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
  }

  async function selectPage(id: string) {
    if (!(await resolveUnsavedChanges())) return;
    try {
      await loadPageIntoEditor(id);
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

  // Ancestor slugs joined into a path, e.g. "homelab/altair/networking".
  function pagePath(pageId: string): string {
    const chain: string[] = [];
    let p: PageSummary | undefined = pages.find((p) => p.id === pageId);
    while (p) {
      chain.unshift(p.slug);
      const parentId: string | null = p.parentId;
      p = parentId ? pages.find((p) => p.id === parentId) : undefined;
    }
    return chain.join("/");
  }

  loadPages();
  loadConfig();
</script>

<div class="layout">
  <aside class="sidebar">
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
        <div class="title-block">
          <input
            class="title-input"
            type="text"
            bind:value={editTitle}
            placeholder="Page title"
          />
          <span class="page-path">~/{pagePath(current.id)}</span>
        </div>
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

  .title-block {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .title-input {
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

  .page-path {
    font-family: var(--font-mono);
    font-size: 0.78rem;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .parent-select {
    background: var(--surface);
    border: 1px solid var(--border);
    color: inherit;
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    max-width: 180px;
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
</style>
