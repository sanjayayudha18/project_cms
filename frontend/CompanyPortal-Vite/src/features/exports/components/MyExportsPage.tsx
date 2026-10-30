import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { MyExportsTable } from "./MyExportsTable";

/** `/exports`: the signed-in user's own export jobs (report roles; no sidebar entry). */
export function MyExportsPage() {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Ekspor"
        title="Ekspor Saya"
        description="Daftar ekspor yang Anda minta. File siap diunduh selama 10 menit dan hanya bisa diunduh satu kali."
      />
      <Card>
        <MyExportsTable />
      </Card>
    </div>
  );
}
