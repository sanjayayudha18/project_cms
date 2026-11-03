/**
 * Types for the admin Users management screen, matching the flat-JSON
 * shape returned by backend/internal/handler/admin_user_handler.go
 * (listUserRowToResponse/getUserRowToResponse/createUserRowToResponse/
 * updateUserRowToResponse). Mounted at /api/v1/admin/users, APPACCESS-only.
 */

export type UserStatus = "active" | "disabled" | "all";
export type AuthSource = "ldap" | "local";

export interface AdminUser {
  id: number;
  username: string;
  full_name: string;
  email: string;
  role: string;
  is_karyawan: boolean;
  auth_source: AuthSource;
  employee_id?: string;
  vendor_id: number | null;
  vendor_branch_id: number | null;
  vendor_branch_name: string | null;
  is_active: boolean;
  deleted_at: string | null;
  supervisor_id: number | null;
  approval_level: number | null;
  last_login_at: string | null;
}

export interface AdminUsersListParams {
  page: number;
  page_size: number;
  status?: UserStatus;
  vendor_id?: number;
  role?: string;
  q?: string;
}

export interface AdminUsersListResponse {
  users: AdminUser[];
  page: number;
  page_size: number;
  total: number;
}

export interface CreateUserPayload {
  username: string;
  full_name: string;
  email: string;
  role: string;
  is_karyawan: boolean;
  auth_source: AuthSource;
  temporary_password?: string;
  employee_id: string;
  vendor_id: number | null;
  vendor_branch_id: number | null;
  supervisor_id: number | null;
  approval_level: number | null;
}

export interface UpdateUserPayload {
  username?: string;
  auth_source?: AuthSource;
  full_name: string;
  email: string;
  role: string;
  is_karyawan: boolean;
  employee_id: string;
  vendor_id: number | null;
  /** null = vendor-wide ("Semua cabang"); selalu dikirim agar cabang lama tidak tertinggal. */
  vendor_branch_id: number | null;
  supervisor_id: number | null;
  approval_level: number | null;
}

/** GET /admin/users/vendor-options (vendor-party-scope Req 2.7). */
export interface VendorOption {
  id: number;
  code: string;
  name: string;
}

export interface VendorBranchOption {
  id: number;
  vendor_id: number;
  branch_code: string;
  branch_name: string;
  region_code: string | null;
}

export interface VendorOptionsResponse {
  vendors: VendorOption[];
  branches: VendorBranchOption[];
}
