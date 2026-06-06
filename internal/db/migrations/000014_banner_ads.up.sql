CREATE TABLE IF NOT EXISTS banner_placements (
    code VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    width INT NOT NULL,
    height INT NOT NULL,
    price_week_kzt INT NOT NULL DEFAULT 0,
    price_month_kzt INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS banner_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    placement_code VARCHAR(64) NOT NULL REFERENCES banner_placements(code),
    recruiter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    image_key VARCHAR(512) NOT NULL DEFAULT '',
    link_url TEXT NOT NULL DEFAULT '',
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    payment_session_id UUID REFERENCES payment_sessions(id) ON DELETE SET NULL,
    amount_kzt INT NOT NULL DEFAULT 0,
    reject_reason TEXT,
    impressions BIGINT NOT NULL DEFAULT 0,
    clicks BIGINT NOT NULL DEFAULT 0,
    priority INT NOT NULL DEFAULT 0,
    expiring_notified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_banner_campaigns_placement_status ON banner_campaigns(placement_code, status);
CREATE INDEX IF NOT EXISTS idx_banner_campaigns_recruiter ON banner_campaigns(recruiter_id);
CREATE INDEX IF NOT EXISTS idx_banner_campaigns_dates ON banner_campaigns(placement_code, starts_at, ends_at)
    WHERE status IN ('pending_review', 'active');

INSERT INTO banner_placements (code, name, description, width, height, price_week_kzt, price_month_kzt) VALUES
    ('home_hero', 'Главная — hero', 'Правый блок на главной странице', 480, 320, 150000, 450000),
    ('jobs_inline', 'Каталог вакансий', 'Между карточками вакансий. Размер: 728×90 или 336×280 px', 728, 90, 80000, 240000),
    ('hackathons_top', 'Раздел хакатонов', 'Верх списка хакатонов', 970, 250, 100000, 300000),
    ('student_dashboard', 'Кабинет студента', 'Блок в личном кабинете студента', 336, 280, 60000, 180000),
    ('freelance_inline', 'Раздел фриланса', 'Между карточками задач', 728, 90, 70000, 210000)
ON CONFLICT (code) DO NOTHING;

INSERT INTO platform_settings (key, value) VALUES (
    'banner_rules',
    '{"max_size_mb":2,"formats":["png","jpg","jpeg","webp"],"rules_text":"Баннер не должен содержать вводящую в заблуждение информацию, оскорбительный контент, политическую рекламу или рекламу конкурирующих платформ."}'::jsonb
) ON CONFLICT (key) DO NOTHING;
