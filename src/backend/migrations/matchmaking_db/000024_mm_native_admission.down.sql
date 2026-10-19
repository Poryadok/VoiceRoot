BEGIN;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM public.mm_native_admission_head) THEN
        RAISE EXCEPTION 'MM native admission marker must be retained';
    END IF;
END $$;
DROP TABLE public.mm_native_admission_head;
COMMIT;
