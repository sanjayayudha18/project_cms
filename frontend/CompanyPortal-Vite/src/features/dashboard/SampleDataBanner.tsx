import { NoticeBanner } from "@/components/ui/NoticeBanner";
import { Info } from "lucide-react";

/** Marks a dashboard widget that still renders static `src/data/*.json` (FE-05). */
export function SampleDataBanner() {
  return (
    <NoticeBanner
      icon={Info}
      variant="info"
      title="Data contoh"
      description="Belum terhubung ke sistem."
    />
  );
}
