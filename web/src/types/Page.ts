export interface PageSummary {
  id: string;
  parentId: string | null;
  slug: string;
  title: string;
}

export interface Page extends PageSummary {
  contentJson: string;
  contentText: string;
  createdAt: string;
  updatedAt: string;
}

export interface Revision {
  id: string;
  pageId: string;
  title: string;
  contentJson: string;
  contentText: string;
  createdAt: string;
}

export interface SearchResult {
  id: string;
  slug: string;
  title: string;
  snippet: string;
}
