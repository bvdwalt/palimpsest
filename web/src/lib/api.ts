import type { Page, PageSummary, Revision, SearchResult } from "../types/Page";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export interface AppConfig {
  autosaveIntervalSeconds: number;
}

export function getConfig(): Promise<AppConfig> {
  return request("/api/config");
}

export function listPages(): Promise<PageSummary[]> {
  return request("/api/pages");
}

export function getPage(id: string): Promise<Page> {
  return request(`/api/pages/${id}`);
}

export function createPage(
  parentId: string | null,
  title: string,
  contentJson = "",
  contentText = "",
): Promise<Page> {
  return request("/api/pages", {
    method: "POST",
    body: JSON.stringify({ parentId, title, contentJson, contentText }),
  });
}

export function updatePage(
  id: string,
  title: string,
  parentId: string | null,
  contentJson: string,
  contentText: string,
): Promise<Page> {
  return request(`/api/pages/${id}`, {
    method: "PUT",
    body: JSON.stringify({ title, parentId, contentJson, contentText }),
  });
}

export function movePage(id: string, parentId: string | null): Promise<Page> {
  return request(`/api/pages/${id}/parent`, {
    method: "PATCH",
    body: JSON.stringify({ parentId }),
  });
}

export function deletePage(id: string): Promise<void> {
  return request(`/api/pages/${id}`, { method: "DELETE" });
}

export function listRevisions(pageId: string): Promise<Revision[]> {
  return request(`/api/pages/${pageId}/revisions`);
}

export function revertToRevision(pageId: string, revisionId: string): Promise<Page> {
  return request(`/api/pages/${pageId}/revert/${revisionId}`, { method: "POST" });
}

export function search(query: string): Promise<SearchResult[]> {
  return request(`/api/search?q=${encodeURIComponent(query)}`);
}
