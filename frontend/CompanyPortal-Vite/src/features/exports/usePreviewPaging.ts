import { useState } from "react";
import { useExportPreview } from "./hooks";
import type { MasterExportParams } from "./types";

/**
 * Keyset paging over `useExportPreview`. The cursor stack starts at `[0]` and resets whenever
 * the Applied_Filter (`params`) changes (Req 2.3); "next" pushes `next_after`, "prev" pops.
 */
export function usePreviewPaging(params: MasterExportParams | null) {
  const key = JSON.stringify(params);
  const [state, setState] = useState({ key, stack: [0] });
  const stack = state.key === key ? state.stack : [0];

  const query = useExportPreview(params, stack[stack.length - 1]);
  const nextAfter = query.data?.next_after ?? null;

  return {
    query,
    pageNumber: stack.length,
    canPrev: stack.length > 1,
    canNext: nextAfter !== null,
    prev: () => setState({ key, stack: stack.slice(0, -1) }),
    next: () => {
      if (nextAfter !== null) setState({ key, stack: [...stack, nextAfter] });
    },
  };
}
