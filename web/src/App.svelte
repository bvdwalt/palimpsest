<script lang="ts">
  import * as api from "./lib/api";
  import type { PageSummary, Page, Revision, SearchResult } from "./types/Page";
  import PageTree from "./components/PageTree.svelte";
  import Editor from "./components/Editor.svelte";
  import RevisionDiff from "./components/RevisionDiff.svelte";

  let pages = $state<PageSummary[]>([]);
  let selectedId = $state<string | null>(null);
  let current = $state<Page | null>(null);

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
    await loadPageIntoEditor(id);
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
  function moveTargets(pageId: string): PageSummary[] {
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
    return pages.filter((p) => !excluded.has(p.id));
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

    <nav>
      <PageTree {pages} parentId={null} {selectedId} onSelect={selectPage} />
    </nav>
  </aside>

  <main>
    {#if current}
      <div class="toolbar">
        <input
          class="title-input"
          type="text"
          bind:value={editTitle}
          placeholder="Page title"
        />
        <select
          class="parent-select"
          value={editParentId ?? ""}
          onchange={onParentSelectChange}
        >
          <option value="">— Top level —</option>
          {#each moveTargets(current.id) as p (p.id)}
            <option value={p.id}>{p.title}</option>
          {/each}
        </select>
        <label class="autosave-toggle">
          <input type="checkbox" bind:checked={autosaveEnabled} />
          Autosave
        </label>
        <button onclick={save} disabled={!dirty || saving}>
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
                    <span>{new Date(rev.createdAt).toLocaleString()} — {rev.title}</span>
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
        <Editor contentJson={editContentJson} onChange={onEditorChange} />
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
    border-right: 1px solid #2a2a2a;
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
    background: #1a1a1a;
    border: 1px solid #333;
    border-radius: 4px;
    color: inherit;
  }

  .search-results {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    z-index: 10;
    background: #1a1a1a;
    border: 1px solid #333;
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
    background: #2a2a2a;
  }

  .search-results .snippet {
    display: block;
    font-size: 0.8rem;
    color: #999;
  }

  .search-results :global(mark) {
    background: #5a4a1a;
    color: inherit;
  }

  .new-page {
    background: #2d4a63;
    border: none;
    color: #fff;
    padding: 0.5rem;
    border-radius: 4px;
    cursor: pointer;
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
  }

  .title-input {
    flex: 1;
    font-size: 1.4rem;
    background: none;
    border: none;
    border-bottom: 1px solid transparent;
    color: inherit;
    padding: 0.25rem 0;
  }

  .title-input:focus {
    outline: none;
    border-bottom-color: #2d4a63;
  }

  .parent-select {
    background: #2a2a2a;
    border: 1px solid #333;
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
    color: #aaa;
    white-space: nowrap;
    cursor: pointer;
  }

  button {
    background: #2a2a2a;
    border: none;
    color: inherit;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
  }

  button:disabled {
    opacity: 0.5;
    cursor: default;
  }

  button.danger {
    background: #5a2a2a;
  }

  .empty-state {
    color: #888;
    margin-top: 3rem;
    text-align: center;
  }

  .error {
    color: #cf6679;
  }

  .revisions ul {
    list-style: none;
    padding: 0;
  }

  .revision-row {
    padding: 0.5rem 0;
    border-bottom: 1px solid #2a2a2a;
  }

  .revision-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .revision-actions {
    display: flex;
    gap: 0.4rem;
    flex-shrink: 0;
  }
</style>
