CREATE TABLE products (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    name         VARCHAR(255)  NOT NULL,
    brand        VARCHAR(255)  NOT NULL DEFAULT '',
    price        DECIMAL(10,2) NOT NULL,
    stock        INT           NOT NULL DEFAULT 0,
    description  VARCHAR(1000) NOT NULL DEFAULT '',
    rating       TINYINT       NOT NULL DEFAULT 0,
    is_new       BOOLEAN       NOT NULL DEFAULT FALSE,
    on_sale      BOOLEAN       NOT NULL DEFAULT FALSE,
    part_type    VARCHAR(45)   NOT NULL,
    manufacturer VARCHAR(100)  NOT NULL DEFAULT '',
    product_line VARCHAR(100)  NOT NULL DEFAULT '',
    model        VARCHAR(100)  NOT NULL DEFAULT '',
    image_key    VARCHAR(255)  NOT NULL DEFAULT '',
    created_at   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_products_price  CHECK (price >= 0),
    CONSTRAINT chk_products_stock  CHECK (stock >= 0),
    CONSTRAINT chk_products_rating CHECK (rating BETWEEN 0 AND 5),
    INDEX idx_products_on_sale (on_sale)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- O carrinho é identificado por um cookie de sessão, sem login.
CREATE TABLE cart_items (
    session_id CHAR(32) NOT NULL,
    product_id BIGINT   NOT NULL,
    quantity   INT      NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (session_id, product_id),
    CONSTRAINT fk_cart_product FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE,
    CONSTRAINT chk_cart_quantity CHECK (quantity > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE orders (
    id             BIGINT AUTO_INCREMENT PRIMARY KEY,
    session_id     CHAR(32)      NOT NULL,
    payment_method VARCHAR(20)   NOT NULL,
    total          DECIMAL(12,2) NOT NULL,
    created_at     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE order_items (
    id           BIGINT AUTO_INCREMENT PRIMARY KEY,
    order_id     BIGINT        NOT NULL,
    product_id   BIGINT        NULL,
    product_name VARCHAR(255)  NOT NULL,
    unit_price   DECIMAL(10,2) NOT NULL,
    quantity     INT           NOT NULL,
    CONSTRAINT fk_order_items_order   FOREIGN KEY (order_id)   REFERENCES orders (id)   ON DELETE CASCADE,
    CONSTRAINT fk_order_items_product FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
