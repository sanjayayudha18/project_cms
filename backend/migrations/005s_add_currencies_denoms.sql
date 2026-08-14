BEGIN;

-- Seed currencies
INSERT INTO currencies (code)
VALUES
    ('IDR'),
    ('USD')
ON CONFLICT (code) DO NOTHING;

-- Seed IDR denoms
INSERT INTO denoms (curr_id, denom)
SELECT c.id, v.denom
FROM currencies c
CROSS JOIN (
    VALUES
        (1000.00),
        (2000.00),
        (5000.00),
        (10000.00),
        (20000.00),
        (50000.00),
        (100000.00)
) AS v(denom)
WHERE c.code = 'IDR'
ON CONFLICT (curr_id, denom) DO NOTHING;

-- Seed USD denoms
INSERT INTO denoms (curr_id, denom)
SELECT c.id, v.denom
FROM currencies c
CROSS JOIN (
    VALUES
        (1.00),
        (5.00),
        (10.00),
        (20.00),
        (50.00),
        (100.00)
) AS v(denom)
WHERE c.code = 'USD'
ON CONFLICT (curr_id, denom) DO NOTHING;

COMMIT;
