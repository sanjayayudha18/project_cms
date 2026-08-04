import { useQuery, useMutation } from '@tanstack/react-query';
import evidenceData from '@/data/evidence.json';
import type { HandoverEvidence } from '@/lib/types';

export function useEvidence(orderId: string) {
  return useQuery({
    queryKey: ['evidence', orderId],
    queryFn: () => {
      const allEvidence = evidenceData as HandoverEvidence[];
      return allEvidence.find((e) => e.orderId === orderId) ?? null;
    },
    enabled: !!orderId,
  });
}

export function useUploadEvidence() {
  return useMutation({
    mutationFn: async (_data: {
      orderId: string;
      files: File[];
      handoverTimestamp: string;
      recipientName: string;
      notes?: string;
    }) => {
      // Simulate upload delay
      await new Promise((resolve) => setTimeout(resolve, 1000));
      return { success: true };
    },
  });
}
