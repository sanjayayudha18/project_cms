import { api } from "@/lib/api/client";
import { useQuery } from "@tanstack/react-query";
import type { DSRUploadRecord } from "../types";

export function useDSRHistory() {
  return useQuery({
    queryKey: ["dsr-uploads"],
    queryFn: async () => {
      const response = await api.get<DSRUploadRecord[]>("/forecasting/dsr/uploads");
      return response.data;
    },
  });
}
