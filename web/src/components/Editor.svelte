<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Editor } from "@tiptap/core";
  import StarterKit from "@tiptap/starter-kit";

  interface Props {
    contentJson: string;
    onChange: (contentJson: string, contentText: string) => void;
  }

  let { contentJson, onChange }: Props = $props();

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
      extensions: [StarterKit],
      content: parseContent(contentJson),
      onUpdate: ({ editor }) => {
        lastSyncedContent = JSON.stringify(editor.getJSON());
        onChange(lastSyncedContent, editor.getText());
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
  }

  .editor :global(.ProseMirror ul),
  .editor :global(.ProseMirror ol) {
    padding-left: 1.5em;
    margin: 0 0 0.75em;
  }

  .editor :global(.ProseMirror pre) {
    background: #1a1a1a;
    border-radius: 4px;
    padding: 0.75em 1em;
    overflow-x: auto;
  }

  .editor :global(.ProseMirror code) {
    background: #1a1a1a;
    border-radius: 3px;
    padding: 0.1em 0.3em;
  }

  .editor :global(.ProseMirror blockquote) {
    border-left: 3px solid #444;
    margin: 0 0 0.75em;
    padding-left: 1em;
    color: #aaa;
  }
</style>
