/** Saves a blob response as a file: name from Content-Disposition, else `fallbackName`. */
export function saveBlobResponse(
  res: { data: Blob; headers: Headers },
  fallbackName: string,
): void {
  const match = /filename="?([^";]+)"?/.exec(res.headers.get("Content-Disposition") ?? "");
  const url = URL.createObjectURL(res.data);
  const a = document.createElement("a");
  a.href = url;
  a.download = match?.[1] ?? fallbackName;
  a.click();
  URL.revokeObjectURL(url);
}
