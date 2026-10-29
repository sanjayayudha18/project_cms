// Per-tab guard so re-renders, repeated polls or a reopened dialog never trigger a second
// auto-download of the same (one-shot) export file (Req 12.13).
const claimed = new Set<number>();

/** true only the first time `jobId` is claimed in this tab. */
export function claimAutoDownload(jobId: number): boolean {
  if (claimed.has(jobId)) return false;
  claimed.add(jobId);
  return true;
}

/** Test helper. */
export function resetAutoDownloadClaims(): void {
  claimed.clear();
}
