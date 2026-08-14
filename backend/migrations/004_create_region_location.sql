BEGIN;

CREATE TABLE IF NOT EXISTS public.regions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code TEXT NOT NULL,
    region TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.locations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    region_id BIGINT NOT NULL,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    rt TEXT,
    rw TEXT,
    kelurahan TEXT,
    kecamatan TEXT,
    city_or_regency TEXT NOT NULL,
    province TEXT NOT NULL,
    postal_code TEXT,
    country_code CHAR(2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_locations_region
        FOREIGN KEY (region_id) REFERENCES public.regions(id)
);

ALTER TABLE public.regions
    ADD CONSTRAINT uq_regions_code UNIQUE (code);

COMMIT;

