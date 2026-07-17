import type { SuggestionOptions } from "@tiptap/suggestion";
import type { PageSummary } from "../types/Page";

const MAX_RESULTS = 8;

// Suggestion config for the `[[` page-link trigger: filters `getPages()` by title,
// renders a plain-DOM popup (no tippy dependency in this repo), and inserts a
// `pageLink` node carrying the page's id + a title snapshot on selection.
// TSelected is left as `any`: Mention's own generic binds it to `MentionNodeAttrs`,
// which doesn't structurally match `PageSummary` even though our `command` below
// only reads `id`/`title` off it.
export function pageLinkSuggestion(
  getPages: () => PageSummary[],
): Omit<SuggestionOptions<PageSummary, any>, "editor"> {
  return {
    char: "[[",
    allowedPrefixes: null,

    items: ({ query }) => {
      const q = query.trim().toLowerCase();
      const pages = getPages();
      const matches = q ? pages.filter((p) => p.title.toLowerCase().includes(q)) : pages;
      return matches.slice(0, MAX_RESULTS);
    },

    command: ({ editor, range, props }) => {
      editor
        .chain()
        .focus()
        .insertContentAt(range, [
          { type: "pageLink", attrs: { id: props.id, label: props.title } },
          { type: "text", text: " " },
        ])
        .run();
    },

    render: () => {
      let popup: HTMLDivElement | null = null;
      let items: PageSummary[] = [];
      let selectedIndex = 0;
      let onSelect: ((item: PageSummary) => void) | null = null;

      function renderItems() {
        if (!popup) return;
        popup.innerHTML = "";

        if (items.length === 0) {
          const empty = document.createElement("div");
          empty.className = "page-link-suggestion-empty";
          empty.textContent = "No matching pages";
          popup.appendChild(empty);
          return;
        }

        items.forEach((item, index) => {
          const el = document.createElement("button");
          el.type = "button";
          el.className = "page-link-suggestion-item";
          el.classList.toggle("is-selected", index === selectedIndex);
          el.textContent = item.title;
          // mousedown (not click) fires before the editor's blur/selection change.
          el.addEventListener("mousedown", (e) => {
            e.preventDefault();
            onSelect?.(item);
          });
          popup!.appendChild(el);
        });
      }

      function positionPopup(clientRect: (() => DOMRect | null) | null | undefined) {
        if (!popup || !clientRect) return;
        const rect = clientRect();
        if (!rect) return;
        popup.style.left = `${rect.left}px`;
        popup.style.top = `${rect.bottom + 4}px`;
      }

      return {
        onStart: (props) => {
          items = props.items;
          selectedIndex = 0;
          onSelect = props.command;
          popup = document.createElement("div");
          popup.className = "page-link-suggestion";
          document.body.appendChild(popup);
          positionPopup(props.clientRect);
          renderItems();
        },

        onUpdate: (props) => {
          items = props.items;
          selectedIndex = 0;
          onSelect = props.command;
          positionPopup(props.clientRect);
          renderItems();
        },

        onKeyDown: ({ event }) => {
          if (event.key === "Escape") {
            popup?.remove();
            popup = null;
            return true;
          }
          if (items.length === 0) return false;
          if (event.key === "ArrowDown") {
            selectedIndex = (selectedIndex + 1) % items.length;
            renderItems();
            return true;
          }
          if (event.key === "ArrowUp") {
            selectedIndex = (selectedIndex - 1 + items.length) % items.length;
            renderItems();
            return true;
          }
          if (event.key === "Enter") {
            onSelect?.(items[selectedIndex]);
            return true;
          }
          return false;
        },

        onExit: () => {
          popup?.remove();
          popup = null;
        },
      };
    },
  };
}
