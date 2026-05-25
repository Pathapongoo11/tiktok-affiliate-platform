-- +goose Up
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    price NUMERIC(10,2),
    commission_rate NUMERIC(5,2),
    category VARCHAR(255),
    shop_product_id VARCHAR(255),
    image_urls JSONB DEFAULT '[]',
    score NUMERIC(4,2) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_products_user_id ON products(user_id);
CREATE INDEX idx_products_category ON products(category);

-- +goose Down
DROP TABLE IF EXISTS products;
