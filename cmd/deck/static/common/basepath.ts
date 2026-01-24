// Injected by server-side template in base.html
declare const basePath: string | undefined;

/**
 * Get the configured base path for Deck deployment
 * @returns The base path (e.g., "/prow") or empty string for root deployment
 */
export function getBasePath(): string {
  return typeof basePath !== 'undefined' ? basePath : '';
}

/**
 * Construct an absolute URL with base path
 * @param path The path relative to base (should start with /)
 * @returns Full URL with protocol, host, base path, and path
 */
export function absoluteURL(path: string): string {
  const base = getBasePath();
  return `${location.protocol}//${location.host}${base}${path}`;
}

/**
 * Construct a path-only URL with base path
 * @param path The path relative to base (should start with /)
 * @returns Path with base path prepended
 */
export function pathURL(path: string): string {
  const base = getBasePath();
  return `${base}${path}`;
}
