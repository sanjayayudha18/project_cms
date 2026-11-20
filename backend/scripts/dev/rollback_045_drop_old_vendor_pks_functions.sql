-- rollback_045_drop_old_vendor_pks_functions.sql
-- Undo backend/migrations/045_drop_old_vendor_pks_functions.sql (spec .kiro/specs/vendor-pks-list/, task 10).
-- DEV ONLY. Run by hand, with the user's OK:
--   psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -f backend/scripts/dev/rollback_045_drop_old_vendor_pks_functions.sql
-- Recreates both functions exactly as 022_vendor_pks.sql defined them. Does not touch schema_migrations:
-- remove the 045 row there yourself if `migrate up` must run 045 again.

BEGIN;

CREATE FUNCTION public.vendor_pks_status(
    p_kind text,
    p_expiry date,
    p_terminated_at timestamptz,
    p_today date,
    p_renewal_months int
) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE
    WHEN p_kind = 'INTERNAL' THEN NULL
    WHEN p_expiry IS NULL THEN 'belum_diisi'
    WHEN p_terminated_at IS NOT NULL THEN 'terminated'
    WHEN p_expiry < p_today THEN 'expired'
    WHEN p_expiry < (p_today + make_interval(months => p_renewal_months))::date THEN 'need_renewal'
    ELSE 'aktif'
  END
$$;

COMMENT ON FUNCTION public.vendor_pks_status(text, date, timestamptz, date, int) IS
    'Label PKS: NULL (INTERNAL), belum_diisi, terminated, expired, need_renewal, aktif. p_today = tanggal Asia/Jakarta dari server (D30); p_renewal_months = VENDOR_PKS_RENEWAL_MONTHS.';

-- Bulan kalender penuh dari p_effective sampai p_expiry + 1 hari (D21): n
-- terbesar dengan (p_effective + n bulan, di-clamp ke akhir bulan) <= p_expiry + 1.
-- Kandidat dari age(), lalu dikoreksi karena age() dan penambahan interval tidak
-- selalu sepakat di akhir bulan. NULL bila salah satu argumen NULL; 0 bila < 1 bulan.
CREATE FUNCTION public.vendor_pks_validity_months(p_effective date, p_expiry date)
RETURNS int
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
  v_end date;
  v_age interval;
  n     int;
BEGIN
  IF p_effective IS NULL OR p_expiry IS NULL THEN
    RETURN NULL;
  END IF;
  v_end := p_expiry + 1;
  v_age := age(v_end::timestamp, p_effective::timestamp);
  n := (extract(year FROM v_age) * 12 + extract(month FROM v_age))::int;
  WHILE n > 0 AND (p_effective + make_interval(months => n))::date > v_end LOOP
    n := n - 1;
  END LOOP;
  WHILE (p_effective + make_interval(months => n + 1))::date <= v_end LOOP
    n := n + 1;
  END LOOP;
  RETURN GREATEST(n, 0);
END
$$;

COMMENT ON FUNCTION public.vendor_pks_validity_months(date, date) IS
    'Masa berlaku PKS dalam bulan kalender penuh (sisa hari dibulatkan ke bawah, D21). 2026-01-01 -> 2027-12-31 = 24.';

COMMIT;
