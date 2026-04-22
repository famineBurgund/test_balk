CREATE TABLE IF NOT EXISTS dislocation_main_events (
    id BIGSERIAL PRIMARY KEY,
    wagon_number TEXT NOT NULL,
    source TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    invoice_number TEXT,
    event_time TIMESTAMPTZ,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dislocation_main_latest
    ON dislocation_main_events (wagon_number, created_at DESC);

CREATE TABLE IF NOT EXISTS dislocation_emd_events (
    id BIGSERIAL PRIMARY KEY,
    wagon_number TEXT NOT NULL,
    source TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    invoice_number TEXT,
    event_time TIMESTAMPTZ,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dislocation_emd_latest
    ON dislocation_emd_events (wagon_number, created_at DESC);

CREATE TABLE IF NOT EXISTS wagon_visits (
    id BIGSERIAL PRIMARY KEY,
    wagon_number TEXT NOT NULL,
    source TEXT NOT NULL,
    invoice_number TEXT,
    pps_number TEXT,
    filed_at TIMESTAMPTZ,
    cleaned_at TIMESTAMPTZ,
    opened_payload JSONB,
    closed_payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wagon_visits_open
    ON wagon_visits (wagon_number, source, cleaned_at);

CREATE TABLE IF NOT EXISTS invoice_batches (
    id BIGSERIAL PRIMARY KEY,
    direction TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO invoice_batches (direction, payload, created_at)
VALUES
(
    'at',
    jsonb_build_object(
        'data', jsonb_build_object(
            'api_processed_intervals', jsonb_build_array(
                jsonb_build_object('from', '2026-04-21 00:00:00', 'to', '2026-04-21 01:00:00'),
                jsonb_build_object('from', '2026-04-21 01:00:00', 'to', '2026-04-21 02:00:00'),
                jsonb_build_object('from', '2026-04-21 02:00:00', 'to', '2026-04-21 03:00:00')
            )
        )
    ),
    '2026-04-21 03:05:00+10'
),
(
    'nmtp',
    jsonb_build_object(
        'data', jsonb_build_object(
            'api_processed_intervals', jsonb_build_array(
                jsonb_build_object('from', '2026-04-21 05:00:00', 'to', '2026-04-21 06:00:00'),
                jsonb_build_object('from', '2026-04-21 06:00:00', 'to', '2026-04-21 07:00:00'),
                jsonb_build_object('from', '2026-04-21 07:00:00', 'to', '2026-04-21 08:00:00')
            )
        )
    ),
    '2026-04-21 08:05:00+10'
);

INSERT INTO dislocation_main_events (wagon_number, source, endpoint, invoice_number, event_time, payload, created_at)
VALUES
(
    '51000001', 'at', 'attis', 'AT00010001', '2026-04-21 08:10:00+10',
    jsonb_build_object('wagon_number', '51000001', 'invoice_number', 'AT00010001', 'station', 'ATTIS-YARD', 'status', 'arrived', 'cargo', 'coal'),
    '2026-04-21 08:10:00+10'
),
(
    '51000001', 'at', 'attis', 'AT00010001', '2026-04-21 12:10:00+10',
    jsonb_build_object('wagon_number', '51000001', 'invoice_number', 'AT00010001', 'station', 'ATTIS-TRACK-4', 'status', 'loading', 'cargo', 'coal'),
    '2026-04-21 12:10:00+10'
),
(
    '51000002', 'at', 'attis', 'AT00010002', '2026-04-21 09:00:00+10',
    jsonb_build_object('wagon_number', '51000002', 'invoice_number', 'AT00010002', 'station', 'ATTIS-YARD', 'status', 'queued', 'cargo', 'ore'),
    '2026-04-21 09:00:00+10'
),
(
    '51000003', 'at', 'filed-cars-at', 'AT00010003', '2026-04-21 11:15:00+10',
    jsonb_build_object('wagon_number', '51000003', 'invoice_number', 'AT00010003', 'pps_number', 'PPS-AT-001', 'status', 'filed'),
    '2026-04-21 11:15:00+10'
),
(
    '52000001', 'nmtp', 'nmtp', 'NM00020001', '2026-04-21 07:20:00+10',
    jsonb_build_object('wagon_number', '52000001', 'invoice_number', 'NM00020001', 'station', 'NMTP-OUTER', 'status', 'arrived', 'cargo', 'containers'),
    '2026-04-21 07:20:00+10'
),
(
    '52000001', 'nmtp', 'nmtp', 'NM00020001', '2026-04-21 10:40:00+10',
    jsonb_build_object('wagon_number', '52000001', 'invoice_number', 'NM00020001', 'station', 'NMTP-PIER-2', 'status', 'processing', 'cargo', 'containers'),
    '2026-04-21 10:40:00+10'
),
(
    '52000002', 'nmtp', 'nmtp', 'NM00020002', '2026-04-21 10:00:00+10',
    jsonb_build_object('wagon_number', '52000002', 'invoice_number', 'NM00020002', 'station', 'NMTP-OUTER', 'status', 'idle', 'cargo', 'grain'),
    '2026-04-21 10:00:00+10'
),
(
    '52000003', 'nmtp', 'pps-status-nmtp', 'NM00020003', '2026-04-21 14:00:00+10',
    jsonb_build_object('wagon_number', '52000003', 'invoice_number', 'NM00020003', 'pps_number', 'PPS-NM-009', 'status', 'cleaned'),
    '2026-04-21 14:00:00+10'
);

INSERT INTO dislocation_emd_events (wagon_number, source, endpoint, invoice_number, event_time, payload, created_at)
VALUES
(
    '53000001', 'ut', 'ut_emd', 'UT00030001', '2026-04-21 06:45:00+10',
    jsonb_build_object('wagon_number', '53000001', 'invoice_number', 'UT00030001', 'station', 'UT-GATE', 'status', 'accepted'),
    '2026-04-21 06:45:00+10'
),
(
    '53000002', 'gut', 'gut_emd', 'GU00030002', '2026-04-21 08:30:00+10',
    jsonb_build_object('wagon_number', '53000002', 'invoice_number', 'GU00030002', 'station', 'GUT-WH-1', 'status', 'accepted'),
    '2026-04-21 08:30:00+10'
),
(
    '53000003', 'at', 'at_emd', 'AE00030003', '2026-04-21 12:50:00+10',
    jsonb_build_object('wagon_number', '53000003', 'invoice_number', 'AE00030003', 'station', 'ATTIS-EMD', 'status', 'loaded'),
    '2026-04-21 12:50:00+10'
),
(
    '53000001', 'ut', 'ut_emd', 'UT00030001', '2026-04-21 15:15:00+10',
    jsonb_build_object('wagon_number', '53000001', 'invoice_number', 'UT00030001', 'station', 'UT-DEPART', 'status', 'sent'),
    '2026-04-21 15:15:00+10'
);

INSERT INTO wagon_visits (wagon_number, source, invoice_number, pps_number, filed_at, cleaned_at, opened_payload, closed_payload, created_at, updated_at)
VALUES
(
    '51000003', 'at', 'AT00010003', 'PPS-AT-001', '2026-04-21 11:15:00+10', NULL,
    jsonb_build_object('wagon_number', '51000003', 'invoice_number', 'AT00010003', 'pps_number', 'PPS-AT-001', 'operation', 'filed'),
    NULL,
    '2026-04-21 11:15:00+10', '2026-04-21 11:15:00+10'
),
(
    '51000004', 'at', 'AT00010004', 'PPS-AT-002', '2026-04-20 09:00:00+10', '2026-04-20 17:00:00+10',
    jsonb_build_object('wagon_number', '51000004', 'invoice_number', 'AT00010004', 'pps_number', 'PPS-AT-002', 'operation', 'filed'),
    jsonb_build_object('wagon_number', '51000004', 'invoice_number', 'AT00010004', 'pps_number', 'PPS-AT-002', 'operation', 'cleaned'),
    '2026-04-20 09:00:00+10', '2026-04-20 17:00:00+10'
),
(
    '51000004', 'at', 'AT00010005', 'PPS-AT-003', '2026-04-21 08:20:00+10', NULL,
    jsonb_build_object('wagon_number', '51000004', 'invoice_number', 'AT00010005', 'pps_number', 'PPS-AT-003', 'operation', 'filed'),
    NULL,
    '2026-04-21 08:20:00+10', '2026-04-21 08:20:00+10'
),
(
    '52000003', 'nmtp', 'NM00020003', 'PPS-NM-009', '2026-04-20 10:30:00+10', '2026-04-21 14:00:00+10',
    jsonb_build_object('wagon_number', '52000003', 'invoice_number', 'NM00020003', 'pps_number', 'PPS-NM-009', 'operation', 'filed'),
    jsonb_build_object('wagon_number', '52000003', 'invoice_number', 'NM00020003', 'pps_number', 'PPS-NM-009', 'operation', 'cleaned'),
    '2026-04-20 10:30:00+10', '2026-04-21 14:00:00+10'
),
(
    '52000004', 'nmtp', 'NM00020004', 'PPS-NM-010', '2026-04-21 12:05:00+10', NULL,
    jsonb_build_object('wagon_number', '52000004', 'invoice_number', 'NM00020004', 'pps_number', 'PPS-NM-010', 'operation', 'filed'),
    NULL,
    '2026-04-21 12:05:00+10', '2026-04-21 12:05:00+10'
);
