let updateInterval;
let isOnline = false;

// 格式化数字
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    } else if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

// 格式化百分比
function formatPercent(num) {
    return num.toFixed(1) + '%';
}

// 格式化时间
function formatDuration(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    
    if (hours > 0) {
        return hours + 'h ' + minutes + 'm ' + secs + 's';
    } else if (minutes > 0) {
        return minutes + 'm ' + secs + 's';
    } else {
        return secs + 's';
    }
}

// 获取状态颜色类
function getStatusClass(value, thresholds) {
    if (value >= thresholds.good) return 'good';
    if (value >= thresholds.warning) return 'warning';
    return 'error';
}

// 创建指标HTML
function createMetric(label, value, unit = '', showProgress = false, progressValue = 0) {
    const progressHtml = showProgress ? 
        `<div class="progress-bar"><div class="progress-fill" style="width: ${progressValue}%"></div></div>` : '';
    
    return `
        <div class="metric">
            <span class="metric-label">${label}</span>
            <span class="metric-value">${value}${unit}</span>
        </div>
        ${progressHtml}
    `;
}

// 更新监控数据
async function updateMonitorData() {
    try {
        const response = await fetch('/api/monitor/data', {
            headers: {
                'User-Agent': 'Mozilla/5.0 (compatible; MediaWarp-Monitor/1.0)'
            }
        });
        if (!response.ok) throw new Error('Network response was not ok');

        const data = await response.json();
        isOnline = true;
        
        document.getElementById('loading').style.display = 'none';
        document.getElementById('error').style.display = 'none';
        document.getElementById('dashboard').style.display = 'grid';
        
        renderDashboard(data);
        updateTimestamp(data.timestamp);
        
    } catch (error) {
        console.error('Error fetching monitor data:', error);
        isOnline = false;
        
        document.getElementById('loading').style.display = 'none';
        document.getElementById('dashboard').style.display = 'none';
        document.getElementById('error').style.display = 'block';
    }
}

// 渲染仪表板
function renderDashboard(data) {
    const dashboard = document.getElementById('dashboard');
    
    dashboard.innerHTML = `
        <!-- 系统状态卡片 -->
        <div class="card">
            <div class="card-title">
                <span class="icon">💻</span>
                系统状态
                <span class="status-indicator ${isOnline ? 'status-online' : 'status-offline'}"></span>
            </div>
            ${createMetric('运行时间', formatDuration(data.system_stats.uptime_seconds))}
            ${createMetric('内存使用', data.system_stats.memory_usage_mb.toFixed(1), 'MB')}
            ${createMetric('协程数量', data.system_stats.goroutine_count)}
            ${createMetric('GC次数', data.system_stats.gc_count)}
        </div>

        <!-- 缓存统计卡片 -->
        <div class="card">
            <div class="card-title">
                <span class="icon">📊</span>
                缓存统计
            </div>
            ${createMetric('总体命中率', formatPercent(data.hit_rates.overall_hit_rate), '', true, data.hit_rates.overall_hit_rate)}
            ${createMetric('总请求数', formatNumber(data.cache_stats.total_requests))}
            ${createMetric('媒体项命中', formatNumber(data.cache_stats.item_info_hits))}
            ${createMetric('Strm类型命中', formatNumber(data.cache_stats.strm_type_hits))}
        </div>

        <!-- 预热统计卡片 -->
        <div class="card">
            <div class="card-title">
                <span class="icon">🔥</span>
                预热统计
            </div>
            ${createMetric('预热状态', data.warmup_stats.enabled ? '✅ 已启用' : '❌ 未启用')}
            ${createMetric('成功率', formatPercent(data.warmup_stats.success_rate), '', true, data.warmup_stats.success_rate)}
            ${createMetric('预热请求', formatNumber(data.warmup_stats.total_warmup_requests))}
            ${createMetric('平均耗时', data.warmup_stats.average_warmup_duration_ms, 'ms')}
        </div>

        <!-- 性能指标卡片 -->
        <div class="card">
            <div class="card-title">
                <span class="icon">⚡</span>
                性能指标
            </div>
            ${createMetric('综合评分', '计算中...', '')}
            ${createMetric('请求速率', '0.00', '/s')}
            ${createMetric('缓存效率', formatPercent(data.hit_rates.overall_hit_rate))}
            ${createMetric('内存效率', '良好', '')}
        </div>

        <!-- 请求去重卡片 -->
        <div class="card">
            <div class="card-title">
                <span class="icon">🔄</span>
                请求去重
            </div>
            ${createMetric('去重状态', data.deduplication_stats.enabled ? '✅ 已启用' : '❌ 未启用')}
            ${data.deduplication_stats.enabled ?
                createMetric('去重率', formatPercent(data.deduplication_stats.deduplication_rate), '', true, data.deduplication_stats.deduplication_rate) +
                createMetric('总请求数', formatNumber(data.deduplication_stats.total_requests)) +
                createMetric('去重次数', formatNumber(data.deduplication_stats.deduplicated_count)) +
                createMetric('节省时间', data.deduplication_stats.saved_time_ms, 'ms')
                : createMetric('状态', '未启用', '')
            }
        </div>
    `;
}

// 更新时间戳
function updateTimestamp(timestamp) {
    const timestampElement = document.getElementById('timestamp');
    const date = new Date(timestamp);
    timestampElement.textContent = '最后更新: ' + date.toLocaleString('zh-CN');
}

// 启动监控
function startMonitoring() {
    updateMonitorData();
    updateInterval = setInterval(updateMonitorData, 3000); // 每3秒更新一次
}

// 停止监控
function stopMonitoring() {
    if (updateInterval) {
        clearInterval(updateInterval);
    }
}

// 页面加载完成后启动监控
document.addEventListener('DOMContentLoaded', startMonitoring);

// 页面卸载时停止监控
window.addEventListener('beforeunload', stopMonitoring);

// 页面可见性变化时控制监控
document.addEventListener('visibilitychange', function() {
    if (document.hidden) {
        stopMonitoring();
    } else {
        startMonitoring();
    }
});
