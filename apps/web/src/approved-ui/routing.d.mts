export function bookmarkedPath(pathname: string, search?: string): string | null;
export function approvedHref(page: string, fid?: string | null): string;
export const lifePages: string[];
export const workspacePages: string[];
export function approvedRoute(hash: string): {page: string; fid?: string | null; path: string; query: URLSearchParams; feature?: any; removed?: boolean; notFound?: boolean; [key: string]: any};
