/** Item table (DMAA read-only context or manual inputs) + "Tambah Baris" for the Vendor Request create page. */

import { Button } from "@/components/ui/Button";
import { formatThousands } from "@/lib/utils/formatCurrency";
import { Plus, Trash2 } from "lucide-react";
import type { FieldArrayWithId, UseFormReturn } from "react-hook-form";
import { type FormValues, displayOrDash, emptyManualItem } from "./vendorRequestCreateForm";

interface VendorRequestCreateItemsTableProps {
  isManual: boolean;
  form: UseFormReturn<FormValues>;
  fields: FieldArrayWithId<FormValues, "items", "id">[];
  onRemove: (index: number) => void;
  onAppend: (item: ReturnType<typeof emptyManualItem>) => void;
}

const TH_CLASS = "px-3 py-2 text-left font-medium text-[var(--n-600)]";
const INPUT_CLASS =
  "rounded-[var(--radius-md)] border border-[var(--n-300)] bg-[var(--n-0)] px-2 py-1 text-sm outline-none focus-visible:border-[var(--red-400)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]";

export function VendorRequestCreateItemsTable({
  isManual,
  form,
  fields,
  onRemove,
  onAppend,
}: VendorRequestCreateItemsTableProps) {
  return (
    <>
      <div className="overflow-x-auto rounded-[var(--radius-lg)] border border-[var(--n-200)]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="border-[var(--n-200)] border-b bg-[var(--n-50)]">
              <th className={TH_CLASS}>ATM ID</th>
              {!isManual && (
                <>
                  <th className={TH_CLASS}>Brand</th>
                  <th className={TH_CLASS}>FLM Vendor</th>
                  <th className={TH_CLASS}>FLM Vendor Region</th>
                </>
              )}
              {isManual && (
                <>
                  <th className={TH_CLASS}>Brand (opsional)</th>
                  <th className={TH_CLASS}>Lokasi ATM (opsional)</th>
                </>
              )}
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">Denominasi</th>
              <th className="px-3 py-2 text-right font-medium text-[var(--n-600)]">
                Amount Replenish
              </th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody>
            {fields.map((field, index) => (
              <tr key={field.id} className="border-[var(--n-100)] border-b last:border-0">
                {isManual ? (
                  <td className="px-3 py-2">
                    <input
                      type="text"
                      aria-label={`ATM ID baris ${index + 1}`}
                      {...form.register(`items.${index}.terminal_id`)}
                      className={`w-32 ${INPUT_CLASS}`}
                    />
                    {form.formState.errors.items?.[index]?.terminal_id && (
                      <div className="mt-1 text-xs text-[var(--danger-fg)]">
                        {form.formState.errors.items[index]?.terminal_id?.message}
                      </div>
                    )}
                  </td>
                ) : (
                  <td className="px-3 py-2">{field.terminal_id}</td>
                )}

                {!isManual && (
                  <>
                    <td className="px-3 py-2 text-[var(--n-700)]">{displayOrDash(field.brand)}</td>
                    <td className="px-3 py-2 text-[var(--n-700)]">
                      {displayOrDash(field.flm_vendor)}
                    </td>
                    <td className="px-3 py-2 text-[var(--n-700)]">
                      {displayOrDash(field.flm_vendor_region)}
                    </td>
                  </>
                )}
                {isManual && (
                  <>
                    <td className="px-3 py-2">
                      <input
                        type="text"
                        aria-label={`Brand baris ${index + 1}`}
                        {...form.register(`items.${index}.brand`)}
                        className={`w-32 ${INPUT_CLASS}`}
                      />
                    </td>
                    <td className="px-3 py-2">
                      <input
                        type="text"
                        aria-label={`Lokasi ATM baris ${index + 1}`}
                        {...form.register(`items.${index}.lokasi_atm`)}
                        className={`w-36 ${INPUT_CLASS}`}
                      />
                    </td>
                  </>
                )}

                {isManual ? (
                  <td className="px-3 py-2 text-right">
                    <input
                      type="number"
                      aria-label={`Denominasi baris ${index + 1}`}
                      {...form.register(`items.${index}.denom`, { valueAsNumber: true })}
                      className={`w-28 text-right tabular-nums ${INPUT_CLASS}`}
                    />
                    {form.formState.errors.items?.[index]?.denom && (
                      <div className="mt-1 text-xs text-[var(--danger-fg)]">
                        {form.formState.errors.items[index]?.denom?.message}
                      </div>
                    )}
                  </td>
                ) : (
                  <td className="px-3 py-2 text-right tabular-nums">
                    {formatThousands(field.denom)}
                  </td>
                )}

                <td className="px-3 py-2 text-right">
                  <input
                    type="number"
                    aria-label={`Amount replenish untuk ${field.terminal_id || `baris ${index + 1}`}`}
                    {...form.register(`items.${index}.amount_replenish`, { valueAsNumber: true })}
                    className={`w-36 text-right tabular-nums ${INPUT_CLASS}`}
                  />
                  {form.formState.errors.items?.[index]?.amount_replenish && (
                    <div className="mt-1 text-xs text-[var(--danger-fg)]">
                      {form.formState.errors.items[index]?.amount_replenish?.message}
                    </div>
                  )}
                </td>
                <td className="px-3 py-2 text-right">
                  <button
                    type="button"
                    onClick={() => onRemove(index)}
                    aria-label={`Hapus baris ${field.terminal_id || index + 1}`}
                    className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-[var(--radius-md)] text-[var(--n-500)] outline-none hover:bg-[var(--n-100)] hover:text-[var(--danger-fg)] focus-visible:ring-2 focus-visible:ring-[var(--red-100)]"
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {form.formState.errors.items?.root && (
        <span className="text-xs text-[var(--danger-fg)]">
          {form.formState.errors.items.root.message}
        </span>
      )}

      {isManual && (
        <Button
          type="button"
          variant="secondary"
          onClick={() => onAppend(emptyManualItem())}
          className="self-start"
        >
          <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
          Tambah Baris
        </Button>
      )}
    </>
  );
}
