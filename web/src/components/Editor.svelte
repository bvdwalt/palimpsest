<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Editor, InputRule } from "@tiptap/core";
  import StarterKit from "@tiptap/starter-kit";
  import Link from "@tiptap/extension-link";
  import Mention from "@tiptap/extension-mention";
  import { pageLinkSuggestion } from "../lib/pageLinkSuggestion";
  import type { PageSummary } from "../types/Page";

  // @tiptap/extension-link only autolinks bare URLs — it has no input rule for
  // typed markdown link syntax, so we add one: typing `[text](url)` converts
  // the whole match to `text` with a Link mark as soon as the `)` is typed.
  const MarkdownLink = Link.extend({
    addInputRules() {
      return [
        new InputRule({
          find: /\[([^\]]+)\]\((\S+)\)$/,
          handler: ({ range, match, chain }) => {
            const [, text, href] = match;
            chain()
              .deleteRange(range)
              .insertContent({ type: "text", marks: [{ type: "link", attrs: { href } }], text })
              .run();
          },
        }),
      ];
    },
  });

  interface Props {
    contentJson: string;
    pages: PageSummary[];
    onChange: (contentJson: string, contentText: string) => void;
    onNavigateToPage: (id: string) => void;
  }

  let { contentJson, pages, onChange, onNavigateToPage }: Props = $props();

  const PageLink = Mention.extend({ name: "pageLink" }).configure({
    HTMLAttributes: { class: "page-link" },
    suggestion: pageLinkSuggestion(() => pages),
    renderText: ({ node }) => node.attrs.label ?? node.attrs.id,
    renderHTML: ({ options, node }) => [
      "a",
      { ...options.HTMLAttributes, href: "#" },
      node.attrs.label ?? node.attrs.id,
    ],
  });

  let element: HTMLDivElement;
  let editor: Editor | undefined;
  let lastSyncedContent: string;

  function parseContent(json: string) {
    if (!json) return "";
    try {
      return JSON.parse(json);
    } catch {
      return "";
    }
  }

  onMount(() => {
    lastSyncedContent = contentJson;
    editor = new Editor({
      element,
      extensions: [
        StarterKit,
        MarkdownLink.configure({
          autolink: true,
          linkOnPaste: true,
          openOnClick: false,
          HTMLAttributes: { target: "_blank", rel: "noopener noreferrer" },
        }),
        PageLink,
      ],
      content: parseContent(contentJson),
      onUpdate: ({ editor }) => {
        lastSyncedContent = JSON.stringify(editor.getJSON());
        onChange(lastSyncedContent, editor.getText());
      },
      editorProps: {
        handleClick: (_view, _pos, event) => {
          const target = (event.target as HTMLElement).closest("a");
          if (!target) return false;
          event.preventDefault();
          const pageId = target.dataset.id;
          if (pageId) {
            onNavigateToPage(pageId);
          } else {
            const href = target.getAttribute("href");
            if (href) window.open(href, "_blank", "noopener,noreferrer");
          }
          return true;
        },
      },
    });
  });

  onDestroy(() => editor?.destroy());

  // Sync on page switch only — onUpdate already owns local keystrokes.
  $effect(() => {
    if (editor && contentJson !== lastSyncedContent) {
      lastSyncedContent = contentJson;
      editor.commands.setContent(parseContent(contentJson));
    }
  });
</script>

<div class="editor" bind:this={element}></div>

<style>
  .editor {
    min-height: 100%;
  }

  .editor :global(.ProseMirror) {
    min-height: 400px;
    outline: none;
    line-height: 1.6;
  }

  .editor :global(.ProseMirror p) {
    margin: 0 0 0.75em;
  }

  .editor :global(.ProseMirror h1),
  .editor :global(.ProseMirror h2),
  .editor :global(.ProseMirror h3) {
    margin: 1em 0 0.5em;
    font-weight: 600;
  }

  .editor :global(.ProseMirror ul),
  .editor :global(.ProseMirror ol) {
    padding-left: 1.5em;
    margin: 0 0 0.75em;
  }

  .editor :global(.ProseMirror pre) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.75em 1em;
    overflow-x: auto;
    font-family: var(--font-mono);
  }

  .editor :global(.ProseMirror code) {
    background: var(--surface);
    border-radius: 3px;
    padding: 0.1em 0.3em;
    font-family: var(--font-mono);
  }

  .editor :global(.ProseMirror pre code) {
    background: none;
    padding: 0;
  }

  .editor :global(.ProseMirror blockquote) {
    border-left: 3px solid var(--accent);
    margin: 0 0 0.75em;
    padding-left: 1em;
    color: var(--muted);
  }

  .editor :global(.ProseMirror a) {
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
  }

  .editor :global(.ProseMirror a.page-link) {
    text-decoration: none;
    border-bottom: 1px dashed var(--accent);
  }

  :global(.page-link-suggestion) {
    position: fixed;
    z-index: 50;
    min-width: 180px;
    max-width: 320px;
    max-height: 240px;
    overflow-y: auto;
    background: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0.25rem;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  }

  :global(.page-link-suggestion-item) {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    color: inherit;
    padding: 0.4rem 0.5rem;
    cursor: pointer;
    border-radius: 3px;
    font-family: inherit;
    font-size: 0.9rem;
  }

  :global(.page-link-suggestion-item.is-selected),
  :global(.page-link-suggestion-item:hover) {
    background: var(--accent-tint);
    color: var(--accent);
  }

  :global(.page-link-suggestion-empty) {
    padding: 0.4rem 0.5rem;
    color: var(--muted);
    font-size: 0.85rem;
  }
</style>
