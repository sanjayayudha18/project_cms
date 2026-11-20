/**
 * Master Data API for Type PKS (/api/v1/admin/pks-types).
 * Endpoints are restricted to ADMIN / ADMIN_PARAM.
 * Writes (create, update, disable, enable) return 202 Accepted (maker-checker).
 */

export {
  listPksTypes,
  submitPksType,
  updatePksType,
  disablePksType,
  enablePksType,
} from "@/features/admin-vendors/pksApi";
