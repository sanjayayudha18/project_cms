import { listDsrUploads } from "@/features/dsr/dsrUploadApi";
import { useQuery } from "@tanstack/react-query";

export function useDsrUploads(date?: string) {
  return useQuery({
    queryKey: ["dsr-uploads", date],
    queryFn: () => listDsrUploads({ dateFrom: date, dateTo: date }),
  });
}
