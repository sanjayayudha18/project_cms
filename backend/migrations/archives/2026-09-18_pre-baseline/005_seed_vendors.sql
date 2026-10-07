-- vendors seed generated from MASTER_ATM_ESQ source
-- normalization applied: BIJAK + Bijak => BIJAK
BEGIN;

INSERT INTO public.vendors (code, name, contact_email, contact_phone, hq_address, is_active)
VALUES
    ('ABACUS', 'Abacus', NULL, NULL, NULL, true),
    ('ADVANTAGE', 'Advantage', NULL, NULL, NULL, true),
    ('BIJAK', 'Bijak', NULL, NULL, NULL, true),
    ('ROH', 'ROH', NULL, NULL, NULL, true),
    ('SSI', 'SSI', NULL, NULL, NULL, true),
    ('TAG', 'TAG', NULL, NULL, NULL, true)
ON CONFLICT (code) DO UPDATE
SET
    name = EXCLUDED.name,
    is_active = EXCLUDED.is_active,
    updated_at = now();

COMMIT;
