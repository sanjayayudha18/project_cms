import { Forbidden } from "@/components/pages/Forbidden";
import { findMasterDataset, parseFilterSearch } from "@/features/exports/datasets";
import { EXPORTS_PAGE_ROLES } from "@/lib/config/exportRoles";
import { Link, createRoute, lazyRouteComponent, useNavigate } from "@tanstack/react-router";
import { SearchX } from "lucide-react";
import { protectedRoute, requireRoles } from "./_protected";

export { EXPORTS_PAGE_ROLES };

const ExportCatalogPage = lazyRouteComponent(
  () => import("@/features/exports/components/ExportCatalogPage"),
  "ExportCatalogPage",
);
const ExportPreviewPage = lazyRouteComponent(
  () => import("@/features/exports/components/ExportPreviewPage"),
  "ExportPreviewPage",
);

export const myExportsRoute = createRoute({
  path: "/exports",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles([...EXPORTS_PAGE_ROLES]),
  loader: () => ExportCatalogPage.preload?.(),
  component: ExportCatalogRouteComponent,
});

function ExportCatalogRouteComponent() {
  const context = myExportsRoute.useRouteContext() as { forbidden?: boolean };
  if (context.forbidden) return <Forbidden />;
  return <ExportCatalogPage />;
}

const FILTER_SEARCH_KEYS = ["status", "vendorId", "branchId", "priceLevel"];

/**
 * The dataset is only known from the path, so validation keeps the raw filter keys and
 * `parseFilterSearch(dataset.id, …)` normalises them in the component (invalid → defaults).
 */
function pickFilterSearch(search: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(search).filter(([k]) => FILTER_SEARCH_KEYS.includes(k)));
}

export const exportPreviewRoute = createRoute({
  path: "/exports/$dataset",
  getParentRoute: () => protectedRoute,
  beforeLoad: requireRoles(["ADMIN", "ADMIN_PARAM"]),
  validateSearch: pickFilterSearch,
  loader: () => ExportPreviewPage.preload?.(),
  component: ExportPreviewRouteComponent,
});

function UnknownDatasetState() {
  return (
    <div className="flex flex-col items-start gap-3 p-6">
      <SearchX className="h-8 w-8 text-[var(--n-500)]" aria-hidden="true" />
      <p className="text-base font-medium text-[var(--n-900)]">Data ekspor tidak dikenal.</p>
      <Link to="/exports" className="font-medium text-[var(--red-600)] underline">
        Kembali ke Export Data
      </Link>
    </div>
  );
}

function ExportPreviewRouteComponent() {
  const context = exportPreviewRoute.useRouteContext() as { forbidden?: boolean };
  const { dataset: slug } = exportPreviewRoute.useParams();
  const search = exportPreviewRoute.useSearch();
  const navigate = useNavigate({ from: exportPreviewRoute.fullPath });
  if (context.forbidden) return <Forbidden />;
  const dataset = findMasterDataset(slug);
  if (!dataset) return <UnknownDatasetState />;
  return (
    <ExportPreviewPage
      // New dataset = fresh draft/paging state.
      key={dataset.id}
      dataset={dataset}
      applied={parseFilterSearch(dataset.id, search)}
      onApplySearch={(next) => void navigate({ search: next })}
    />
  );
}
