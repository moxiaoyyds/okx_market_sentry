-- OKX Market Sentry 数据库初始化脚本
-- 用于唐奇安通道量化策略的数据表创建

-- 使用指定数据库
USE okx_strategy;

-- K线数据表
CREATE TABLE IF NOT EXISTS klines (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL COMMENT '交易对符号',
    open_time BIGINT NOT NULL COMMENT '开盘时间戳(毫秒)',
    close_time BIGINT NOT NULL COMMENT '收盘时间戳(毫秒)',
    open_price DECIMAL(20,8) NOT NULL COMMENT '开盘价',
    high_price DECIMAL(20,8) NOT NULL COMMENT '最高价',
    low_price DECIMAL(20,8) NOT NULL COMMENT '最低价',
    close_price DECIMAL(20,8) NOT NULL COMMENT '收盘价',
    volume DECIMAL(20,8) NOT NULL COMMENT '成交量',
    quote_volume DECIMAL(20,8) DEFAULT 0 COMMENT '成交额',
    created_at TIMESTAMP NULL DEFAULT NULL COMMENT '创建时间',
    updated_at TIMESTAMP NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    
    -- 索引优化
    INDEX idx_symbol_time (symbol, open_time),
    INDEX idx_open_time (open_time),
    INDEX idx_created_at (created_at),
    
    -- 唯一约束：同一交易对同一时间点只能有一条K线数据
    UNIQUE KEY uk_symbol_time (symbol, open_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='K线数据表';

-- 交易信号表
CREATE TABLE IF NOT EXISTS trading_signals (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    signal_id VARCHAR(50) NOT NULL COMMENT '信号唯一标识',
    symbol VARCHAR(20) NOT NULL COMMENT '交易对符号',
    signal_type ENUM('LONG', 'SHORT') NOT NULL COMMENT '信号类型',
    signal_strength DECIMAL(5,4) NOT NULL COMMENT '信号强度(0-1)',
    
    -- 价格信息
    trigger_price DECIMAL(20,8) NOT NULL COMMENT '触发价格',
    breakout_amplitude DECIMAL(10,6) NOT NULL COMMENT '突破幅度(%)',
    
    -- 技术指标数据
    donchian_upper DECIMAL(20,8) NOT NULL COMMENT '唐奇安通道上轨',
    donchian_lower DECIMAL(20,8) NOT NULL COMMENT '唐奇安通道下轨',
    atr_value DECIMAL(20,8) NOT NULL COMMENT 'ATR值',
    atr_slope DECIMAL(10,6) NOT NULL COMMENT 'ATR斜率',
    
    -- 成交量数据
    trigger_volume DECIMAL(20,8) NOT NULL COMMENT '触发时成交量',
    volume_ratio DECIMAL(10,6) NOT NULL COMMENT '成交量比率',
    
    -- 市场状态
    consolidation_bars INT NOT NULL COMMENT '盘整K线数量',
    market_volatility DECIMAL(10,6) DEFAULT 0 COMMENT '市场波动率',
    
    -- 时间信息
    trigger_time BIGINT NOT NULL COMMENT '信号触发时间戳',
    kline_time BIGINT NOT NULL COMMENT 'K线时间戳',
    created_at TIMESTAMP NULL DEFAULT NULL COMMENT '创建时间',
    
    -- 索引优化
    INDEX idx_symbol (symbol),
    INDEX idx_signal_type (signal_type),
    INDEX idx_trigger_time (trigger_time),
    INDEX idx_signal_strength (signal_strength),
    INDEX idx_created_at (created_at),
    
    -- 唯一约束：防止重复信号
    UNIQUE KEY uk_signal_id (signal_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='交易信号表';

-- 性能监控表
CREATE TABLE IF NOT EXISTS performance_metrics (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    metric_name VARCHAR(50) NOT NULL COMMENT '指标名称',
    metric_value DECIMAL(20,8) NOT NULL COMMENT '指标值',
    metric_unit VARCHAR(20) DEFAULT '' COMMENT '指标单位',
    category VARCHAR(30) NOT NULL COMMENT '指标分类',
    description TEXT NULL COMMENT '指标描述',
    created_at TIMESTAMP NULL DEFAULT NULL COMMENT '创建时间',
    
    -- 索引优化
    INDEX idx_metric_name (metric_name),
    INDEX idx_category (category),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='性能监控指标表';

-- 插入初始的性能监控指标
INSERT IGNORE INTO performance_metrics (metric_name, metric_value, metric_unit, category, description) VALUES
('websocket_connections', 0, 'count', 'connection', 'WebSocket连接数量'),
('database_connections', 0, 'count', 'database', '数据库连接数量'),
('signal_quality_score', 0, 'score', 'strategy', '信号质量评分'),
('data_processing_latency', 0, 'ms', 'performance', '数据处理延迟'),
('memory_usage', 0, 'MB', 'system', '内存使用量'),
('cpu_usage', 0, '%', 'system', 'CPU使用率');

-- 创建用户权限 (如果用户不存在)
CREATE USER IF NOT EXISTS 'okx_user'@'%' IDENTIFIED BY 'okx123456';
GRANT SELECT, INSERT, UPDATE, DELETE ON okx_strategy.* TO 'okx_user'@'%';
FLUSH PRIVILEGES;

-- 显示创建的表
SHOW TABLES;

-- 显示表结构信息
DESCRIBE klines;
DESCRIBE trading_signals;
DESCRIBE performance_metrics;